# coco-iam API (Go)

Dieses Verzeichnis enthält die Go-basierte API von coco-iam. Unten findest du Schritte zum Starten/Stoppen der Anwendung sowie zum Anlegen eines Superadmin-Users.

## Voraussetzungen
- Go 1.21+ installiert
- macOS (zsh) oder kompatible Shell
- Schreibrechte für `data/`-Verzeichnis (Logs/DB)

## Konfiguration
Standardmäßig wird die API mit `./config.json` gestartet. Du kannst einen anderen Pfad als zweites Argument übergeben.

## Anwendung starten
Startet den HTTP-Server, führt Migrationen aus und prüft, ob ein Superadmin existiert.

```zsh
cd /Users/andet/Desktop/Development/playground/go/coco-iam/api
# mit Standard-Konfig
go run . start

# mit eigener Konfigurationsdatei
go run . start /Pfad/zu/deiner/config.json
```

Beim Start werden:
- Logger initialisiert (Logs unter `data/logs/<YYYY>/<MM>/<DD>/`)
- Datenbankmigrationen aus dem eingebetteten Paket ausgeführt
- Der Server gestartet und eine PID-Datei angelegt (Pfad in der Konfiguration `PidFile`)

## Anwendung stoppen
Beendet den laufenden Server-Prozess über die PID-Datei.

```zsh
cd /Users/andet/Desktop/Development/playground/go/coco-iam/api
# mit Standard-Konfig
go run . shutdown

# mit eigener Konfigurationsdatei
go run . shutdown /Pfad/zu/deiner/config.json
```

Hinweis: Die PID-Datei wird über die Konfiguration verwaltet. Falls kein `PidFile` gesetzt ist, wird `./server.pid` verwendet.

## Superadmin erstellen
Legt einen aktiven Superadmin-User an. Die Argumente sind Pflicht: Benutzername, E-Mail und Passwort.

```zsh
cd /Users/andet/Desktop/Development/playground/go/coco-iam/api
# Syntax
go run . create-admin <username> <email> <password>

# Beispiel
go run . create-admin admin admin@example.com MyStrongPassword123
```

Wichtig:
- Der interaktive Modus ist deaktiviert. Ohne alle drei Argumente bricht der Befehl mit Fehler ab.
- Das Passwort wird aktuell nur validiert (nicht leer), aber nicht gehasht/gespeichert, sofern die Entität/Repository kein entsprechendes Feld/Verhalten vorsieht. Wenn du Passwort-Hashing (z. B. bcrypt) brauchst, ergänze dies in `api/config/db/install.go` innerhalb `AddSuperadminWithArgs`.

## Logs und Daten
- Datenbank: `./data/db/users.db`
- Logs: `./data/logs/<YYYY>/<MM>/<DD>/server_*.log`

## Entwicklung
Zum schnellen Testen:

```zsh
cd /Users/andet/Desktop/Development/playground/go/coco-iam/api
# bauen
go build ./...
# laufen lassen (Start)
./api start
```

Oder direkt:

```zsh
cd /Users/andet/Desktop/Development/playground/go/coco-iam/api
go run . start
```

## Fehlerbehebung
- "DatabaseManager or Connector is nil": Prüfe Schreibrechte und dass das `data/db`-Verzeichnis existiert.
- "No superadmin user found": Erstelle einen Superadmin mit `create-admin`.
- Ports/Logs: Prüfe `config.json` auf Port und PID-File-Einstellungen.
