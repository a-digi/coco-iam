package updater

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/security/geoip"
)

// countryInfo is one row of GeoLite2-Country-Locations-en.csv, keyed
// by geoname_id — joined against each row of the blocks CSVs, which
// only carry the geoname_id, not the country code/name directly.
type countryInfo struct {
	code string
	name string
}

// columnIndex maps a CSV header row to column position, so the
// parsers below read by column name rather than positional index —
// MaxMind's own CSV column order isn't part of any documented
// contract, only the names are.
func columnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, name := range header {
		idx[name] = i
	}
	return idx
}

// cidrRange returns the first and last address covered by cidr (e.g.
// "1.2.3.0/24"), as net.IP values of the same length as the network's
// own address family.
func cidrRange(cidr string) (start, end net.IP, err error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	start = ipnet.IP
	end = make(net.IP, len(ipnet.IP))
	for i := range ipnet.IP {
		end[i] = ipnet.IP[i] | ^ipnet.Mask[i]
	}
	return start, end, nil
}

// parseCountryLocations reads GeoLite2-Country-Locations-en.csv into
// a geoname_id -> countryInfo map.
func parseCountryLocations(r io.Reader) (map[string]countryInfo, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	col := columnIndex(header)
	geonameIdx, ok := col["geoname_id"]
	if !ok {
		return nil, fmt.Errorf("missing geoname_id column")
	}
	codeIdx, ok := col["country_iso_code"]
	if !ok {
		return nil, fmt.Errorf("missing country_iso_code column")
	}
	nameIdx, ok := col["country_name"]
	if !ok {
		return nil, fmt.Errorf("missing country_name column")
	}

	locations := make(map[string]countryInfo)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		locations[row[geonameIdx]] = countryInfo{code: row[codeIdx], name: row[nameIdx]}
	}
	return locations, nil
}

// importCountryBlocks streams one GeoLite2-Country-Blocks-IPv{4,6}.csv,
// joins each row against locations by geoname_id (falling back to
// registered_country_geoname_id when geoname_id is blank — MaxMind
// leaves geoname_id empty for anonymizing-proxy/satellite-provider
// rows but still carries a registered country), and inserts into
// geoip_country_ranges within a single transaction. Returns the
// number of rows inserted. A row with an unparseable network is
// skipped rather than aborting the whole import — one bad line in a
// few-hundred-thousand-row file shouldn't sink the entire pull.
func importCountryBlocks(db *sql.DB, family string, locations map[string]countryInfo, r io.Reader) (int, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}
	col := columnIndex(header)
	networkIdx, ok := col["network"]
	if !ok {
		return 0, fmt.Errorf("missing network column")
	}
	geonameIdx, hasGeoname := col["geoname_id"]
	registeredIdx, hasRegistered := col["registered_country_geoname_id"]

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO geoip_country_ranges (family, start_ip, end_ip, country_code, country_name) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = tx.Rollback()
			return inserted, fmt.Errorf("read row: %w", err)
		}

		start, end, err := cidrRange(row[networkIdx])
		if err != nil {
			continue
		}
		_, startHex, ok1 := geoip.EncodeIP(start)
		_, endHex, ok2 := geoip.EncodeIP(end)
		if !ok1 || !ok2 {
			continue
		}

		var geonameID string
		if hasGeoname {
			geonameID = row[geonameIdx]
		}
		if geonameID == "" && hasRegistered {
			geonameID = row[registeredIdx]
		}
		loc := locations[geonameID]

		if _, err := stmt.Exec(family, startHex, endHex, loc.code, loc.name); err != nil {
			_ = tx.Rollback()
			return inserted, fmt.Errorf("insert row: %w", err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}

// importASNBlocks streams one GeoLite2-ASN-Blocks-IPv{4,6}.csv into
// geoip_asn_ranges — same single-transaction, skip-bad-rows discipline
// as importCountryBlocks.
func importASNBlocks(db *sql.DB, family string, r io.Reader) (int, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}
	col := columnIndex(header)
	networkIdx, ok := col["network"]
	if !ok {
		return 0, fmt.Errorf("missing network column")
	}
	asnIdx, ok := col["autonomous_system_number"]
	if !ok {
		return 0, fmt.Errorf("missing autonomous_system_number column")
	}
	orgIdx, ok := col["autonomous_system_organization"]
	if !ok {
		return 0, fmt.Errorf("missing autonomous_system_organization column")
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO geoip_asn_ranges (family, start_ip, end_ip, asn, as_org) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = tx.Rollback()
			return inserted, fmt.Errorf("read row: %w", err)
		}

		start, end, err := cidrRange(row[networkIdx])
		if err != nil {
			continue
		}
		_, startHex, ok1 := geoip.EncodeIP(start)
		_, endHex, ok2 := geoip.EncodeIP(end)
		if !ok1 || !ok2 {
			continue
		}

		asn, err := strconv.Atoi(row[asnIdx])
		if err != nil {
			continue
		}

		if _, err := stmt.Exec(family, startHex, endHex, asn, row[orgIdx]); err != nil {
			_ = tx.Rollback()
			return inserted, fmt.Errorf("insert row: %w", err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}

// importCountryCSVDir locates and imports the full country dataset
// (locations + both IPv4/IPv6 block files) somewhere under dir —
// MaxMind's zip may or may not wrap the CSVs in a versioned
// subdirectory, so this searches rather than assumes a fixed layout.
func importCountryCSVDir(db *sql.DB, dir string) (int, error) {
	locationsPath, err := findCSVFile(dir, "GeoLite2-Country-Locations-en.csv")
	if err != nil {
		return 0, err
	}
	locFile, err := os.Open(locationsPath)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", locationsPath, err)
	}
	locations, err := parseCountryLocations(locFile)
	_ = locFile.Close()
	if err != nil {
		return 0, fmt.Errorf("parse locations: %w", err)
	}

	total := 0
	for _, spec := range []struct{ family, filename string }{
		{"v4", "GeoLite2-Country-Blocks-IPv4.csv"},
		{"v6", "GeoLite2-Country-Blocks-IPv6.csv"},
	} {
		blocksPath, err := findCSVFile(dir, spec.filename)
		if err != nil {
			return total, err
		}
		blocksFile, err := os.Open(blocksPath)
		if err != nil {
			return total, fmt.Errorf("open %s: %w", blocksPath, err)
		}
		n, err := importCountryBlocks(db, spec.family, locations, blocksFile)
		_ = blocksFile.Close()
		if err != nil {
			return total, fmt.Errorf("import %s: %w", spec.filename, err)
		}
		total += n
	}
	return total, nil
}

// importASNCSVDir mirrors importCountryCSVDir for the ASN dataset.
func importASNCSVDir(db *sql.DB, dir string) (int, error) {
	total := 0
	for _, spec := range []struct{ family, filename string }{
		{"v4", "GeoLite2-ASN-Blocks-IPv4.csv"},
		{"v6", "GeoLite2-ASN-Blocks-IPv6.csv"},
	} {
		blocksPath, err := findCSVFile(dir, spec.filename)
		if err != nil {
			return total, err
		}
		blocksFile, err := os.Open(blocksPath)
		if err != nil {
			return total, fmt.Errorf("open %s: %w", blocksPath, err)
		}
		n, err := importASNBlocks(db, spec.family, blocksFile)
		_ = blocksFile.Close()
		if err != nil {
			return total, fmt.Errorf("import %s: %w", spec.filename, err)
		}
		total += n
	}
	return total, nil
}

// findCSVFile searches the tree rooted at dir for a file named name,
// returning its path. Errors if it's not found anywhere.
func findCSVFile(dir, name string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == name {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search for %s under %s: %w", name, dir, err)
	}
	if found == "" {
		return "", fmt.Errorf("%s not found under %s", name, dir)
	}
	return found, nil
}

// unzip extracts every file in the zip archive at zipPath into
// destDir, rejecting any entry whose name would resolve outside
// destDir (zip-slip path traversal) — defense in depth even though
// MaxMind is a trusted source.
func unzip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir)
	for _, f := range r.File {
		path := filepath.Join(destDir, f.Name)
		if path != cleanDest && !strings.HasPrefix(path, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry escapes destination directory: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", path, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
		}
		if err := extractZipEntry(f, path); err != nil {
			return fmt.Errorf("extract %s: %w", f.Name, err)
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
