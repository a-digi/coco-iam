package xbrl

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/a-digi/coco-filer/filer"
	"github.com/a-digi/coco-iam/src/xbrl/standard/entity"
	"github.com/a-digi/coco-iam/src/xbrl/standard/repository"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type ImportRequest struct {
	Data string `json:"data"`
}

type ImportHandler struct{}

func calculateFileMD5Hash(file io.ReadSeeker) (string, error) {
	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	return hash, nil
}

func duplicateFileResponse(w http.ResponseWriter, fileHash string) {
	message := "File with this hash already exists. Hash: " + fileHash
	response.ErrorResponse(w, http.StatusConflict, message)
}

func (h *ImportHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	var resp = map[string]interface{}{"status": "imported"}
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		ctx.GetLogger().Error("ImportHandler: failed to parse multipart form: %v", err)
		response.ErrorResponse(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		ctx.GetLogger().Error("ImportHandler: form file retrieval error: %v", err)
		response.ErrorResponse(w, http.StatusBadRequest, "File not provided")
		return
	}
	if handler.Filename == "" || handler.Size == 0 {
		ctx.GetLogger().Error("ImportHandler: no file uploaded or empty file: name='%s', size=%d", handler.Filename, handler.Size)
		response.ErrorResponse(w, http.StatusBadRequest, "No file uploaded")
		return
	}

	defer file.Close()

	fileHash, hashErr := calculateFileMD5Hash(file)

	if hashErr != nil {
		ctx.GetLogger().Error("ImportHandler: error hashing file: %v", hashErr)
		response.ErrorResponse(w, http.StatusInternalServerError, "Could not hash file")
		return
	}

	// Check if file hash exists in the database
	repo := repository.NewStandardRepository(ctx.GetDatabaseManager())
	std, err := repo.FindByFileHash(fileHash)
	if err != nil {
		ctx.GetLogger().Error("ImportHandler: error checking file hash in DB: %v", err)
		response.ErrorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}

	if std != nil {
		duplicateFileResponse(w, fileHash)
		return
	}

	mpm := filer.NewFileMultiPartManager("")
	fileInfo, saveErr := mpm.MoveFile(file, handler)
	if saveErr != nil {
		ctx.GetLogger().Error("ImportHandler: error saving uploaded file: %v", saveErr)
		response.ErrorResponse(w, http.StatusInternalServerError, "Could not save file")
		return
	}

	resp["file"] = fileInfo
	resp["file_hash"] = fileHash

	// Formulardaten in Standard-Struktur mappen
	standard := &entity.Standard{
		FilePath: fileInfo.Path,
		FileHash: fileHash,
	}
	if err := request.MapFormToStruct(r, standard); err != nil {
		ctx.GetLogger().Error("ImportHandler: error mapping form to struct: %v", err)
		response.ErrorResponse(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	repo.Insert(standard)
	resp["standard"] = standard

	response.SuccessResponse(w, http.StatusOK, resp)
}
