package project

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// Manifest holds a parsed desi.mod manifest.
type Manifest struct {
	Package     Package
	Build       Build
	Target      Target
	Diagnostics Diagnostics
	FFI         FFI
	Database    Database            // [database] → the "default" connection
	Databases   map[string]Database // [database.name] → named connections
	path        string              // absolute path to manifest
}

type Package struct {
	Name    string
	Version string
	Edition string
	Entry   string
	Roots   []string
}

type Build struct {
	Mode   string
	OutDir string
}

type Target struct {
	Triple string
}

type Diagnostics struct {
	ErrorFormat string // human|json
	Color       string // auto|always|never
	MaxErrors   string // stored as string; parsed/validated to int
}

type FFI struct {
	Libs    []string
	Search  []string
	Externs []Extern
}

type Extern struct {
	Name string
	Lib  string
}

type Database struct {
	Engine       string // "postgres" or "mysql" (required)
	SchemaOnly   bool   // if true, no connection fields needed
	DebugQueries bool   // if true, print SQL + params to stderr
	Host         string // DB hostname or IP address
	Port         string // DB port (auto-defaults: 5432/postgres, 3306/mysql)
	Name         string // database name
	User         string // DB username
	Password     string // DB password
	SslMode      string // TLS mode: "disable", "require", "verify-ca", "verify-full"
	Charset      string // character encoding: "utf8mb4", "UTF8"
	Timezone     string // connection timezone: "UTC", "America/Chicago"
	Prefix       string // table name prefix for multi-tenancy: "app1_"
	MaxConns     string // max pool connections (default: "1" = no pool)
	ConnTimeout  string // connection timeout in seconds (default: "30")
	Options      string // extra DSN/connection string parameters
}

// EntryPath resolves the absolute entry file (relative to manifest dir).
func (m Manifest) EntryPath() string {
	if m.Package.Entry == "" || m.path == "" {
		return ""
	}
	if filepath.IsAbs(m.Package.Entry) {
		return m.Package.Entry
	}
	return filepath.Join(filepath.Dir(m.path), filepath.Clean(m.Package.Entry))
}

func (m Manifest) Roots() []string {
	rs := make([]string, 0, len(m.Package.Roots))
	base := filepath.Dir(m.path)
	for _, r := range m.Package.Roots {
		if filepath.IsAbs(r) {
			rs = append(rs, filepath.Clean(r))
		} else {
			rs = append(rs, filepath.Join(base, filepath.Clean(r)))
		}
	}
	return rs
}

func (m Manifest) OutDir() string {
	if m.Build.OutDir == "" || m.path == "" {
		return ""
	}
	if filepath.IsAbs(m.Build.OutDir) {
		return filepath.Clean(m.Build.OutDir)
	}
	return filepath.Join(filepath.Dir(m.path), filepath.Clean(m.Build.OutDir))
}

type DiagDefaults struct {
	ErrorFormat string
	Color       string
	MaxErrors   int // 0 => unspecified
}

func (m Manifest) DiagDefaults() DiagDefaults {
	n := 0
	if m.Diagnostics.MaxErrors != "" {
		if v, err := strconv.Atoi(m.Diagnostics.MaxErrors); err == nil && v > 0 {
			n = v
		}
	}
	return DiagDefaults{
		ErrorFormat: strings.ToLower(m.Diagnostics.ErrorFormat),
		Color:       strings.ToLower(m.Diagnostics.Color),
		MaxErrors:   n,
	}
}

// FindRoot walks up from startDir to find a directory containing desi.mod.
func FindRoot(startDir string) (rootDir, manifestPath string, ok bool) {
	dir := startDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", "", false
		}
	}
	for {
		mp := filepath.Join(dir, "desi.mod")
		if st, err := os.Stat(mp); err == nil && !st.IsDir() {
			return dir, mp, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// reached filesystem root
			return "", "", false
		}
		dir = parent
	}
}

// Load parses and validates a manifest file.
func Load(path string) (Manifest, []diag.Diagnostic) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, []diag.Diagnostic{simpleDiag("project.io_error", "DPM9999", path, fmt.Sprintf("cannot read %s", path))}
	}
	m, diags := parseDML(string(raw))
	m.path = filepath.Clean(path)
	// Validation
	mdir := filepath.Dir(m.path)
	// Required: entry
	if m.Package.Entry == "" {
		diags = append(diags, simpleDiag("project.missing_entry", "DPM0003", path, "missing [package].entry"))
	} else {
		if !filepath.IsAbs(m.Package.Entry) {
			target := filepath.Join(mdir, m.Package.Entry)
			if _, err := os.Stat(target); err != nil {
				// not fatal, but helpful
				diags = append(diags, simpleDiag("project.entry_not_found", "DPM0006", path, fmt.Sprintf("entry not found: %s", target)))
			}
		}
	}
	// Validate roots exist
	for _, r := range m.Package.Roots {
		check := r
		if !filepath.IsAbs(check) {
			check = filepath.Join(mdir, r)
		}
		if st, err := os.Stat(check); err != nil || !st.IsDir() {
			diags = append(diags, simpleDiag("project.bad_root", "DPM0004", path, fmt.Sprintf("non-existent root: %s", check)))
		}
	}
	// Validate diagnostics enums
	if m.Diagnostics.ErrorFormat != "" {
		switch strings.ToLower(m.Diagnostics.ErrorFormat) {
		case "human", "json":
		default:
			diags = append(diags, simpleDiag("project.bad_diag_value", "DPM0005", path, "invalid diagnostics.error_format (expected human|json)"))
		}
	}
	if m.Diagnostics.Color != "" {
		switch strings.ToLower(m.Diagnostics.Color) {
		case "auto", "always", "never":
		default:
			diags = append(diags, simpleDiag("project.bad_diag_value", "DPM0005", path, "invalid diagnostics.color (expected auto|always|never)"))
		}
	}
	if m.Diagnostics.MaxErrors != "" {
		if _, err := strconv.Atoi(m.Diagnostics.MaxErrors); err != nil {
			diags = append(diags, simpleDiag("project.bad_diag_type", "DPM0002", path, "diagnostics.max_errors must be a string containing an integer"))
		}
	}
	// Validate [database] section (default)
	diags = append(diags, validateDatabase(&m.Database, "database", path)...)
	// Validate named [database.name] sections
	for name := range m.Databases {
		entry := m.Databases[name]
		diags = append(diags, validateDatabase(&entry, "database."+name, path)...)
		m.Databases[name] = entry
	}
	return m, diags
}

// validateDatabase validates a single Database entry and applies defaults.
// secName is used for diagnostic messages (e.g., "database" or "database.analytics").
func validateDatabase(db *Database, secName, path string) []diag.Diagnostic {
	var diags []diag.Diagnostic
	if db.Engine == "" {
		return nil // no database configured in this section
	}
	switch strings.ToLower(db.Engine) {
	case "postgres", "mysql":
		db.Engine = strings.ToLower(db.Engine)
	default:
		diags = append(diags, simpleDiag("project.bad_db_engine", "DPM0007", path,
			fmt.Sprintf("invalid %s.engine %q (expected postgres|mysql)", secName, db.Engine)))
	}
	// If not schema_only, require connection fields
	if !db.SchemaOnly {
		if db.Host == "" {
			diags = append(diags, simpleDiag("project.missing_db_field", "DPM0008", path,
				fmt.Sprintf("%s.host required (or set schema_only = true)", secName)))
		}
		if db.Name == "" {
			diags = append(diags, simpleDiag("project.missing_db_field", "DPM0008", path,
				fmt.Sprintf("%s.name required (or set schema_only = true)", secName)))
		}
		if db.User == "" {
			diags = append(diags, simpleDiag("project.missing_db_field", "DPM0008", path,
				fmt.Sprintf("%s.user required (or set schema_only = true)", secName)))
		}
	}
	// Auto-default port based on engine
	if db.Port == "" {
		switch db.Engine {
		case "postgres":
			db.Port = "5432"
		case "mysql":
			db.Port = "3306"
		}
	}
	// Validate ssl_mode if provided
	if db.SslMode != "" {
		switch strings.ToLower(db.SslMode) {
		case "disable", "require", "verify-ca", "verify-full", "prefer", "allow":
			db.SslMode = strings.ToLower(db.SslMode)
		default:
			diags = append(diags, simpleDiag("project.bad_db_ssl", "DPM0009", path,
				fmt.Sprintf("invalid %s.ssl_mode %q (expected disable|require|verify-ca|verify-full)", secName, db.SslMode)))
		}
	}
	return diags
}

// ---- Minimal DML (Desi Manifest Language) parser ----
// Supports: # comments, [section], [[section.sub]], key = "str", key = ["a","b"], booleans, and bare words for enums.
// Only string/boolean/array-of-strings are materialized; other types are reported as invalid type.

type dmlState struct {
	section string // e.g., "package", "build", "target", "diagnostics", "ffi", "ffi.extern", "database", "database.analytics"
	dbName  string // non-empty when inside a [database.name] sub-section
	line    int
}

func parseDML(src string) (Manifest, []diag.Diagnostic) {
	m := Manifest{}
	var diags []diag.Diagnostic
	s := dmlState{}
	sc := bufio.NewScanner(strings.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		s.line++
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		// section headers
		if strings.HasPrefix(trim, "[[") && strings.HasSuffix(trim, "]]") {
			body := strings.TrimSpace(trim[2 : len(trim)-2])
			if body == "ffi.extern" {
				s.section = "ffi.extern"
			} else {
				diags = append(diags, simpleDiag("project.unknown_section", "DPM0001", "", fmt.Sprintf("unknown array-table section: %s", body)))
			}
			continue
		}
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			body := strings.TrimSpace(trim[1 : len(trim)-1])
			switch body {
			case "package", "build", "target", "diagnostics", "ffi", "database":
				s.section = body
				s.dbName = "" // reset named DB context
			default:
				// Check for [database.name] sub-sections
				if strings.HasPrefix(body, "database.") {
					dbName := strings.TrimPrefix(body, "database.")
					if dbName != "" {
						s.section = "database"
						s.dbName = dbName
						// Initialize map and entry if needed
						if m.Databases == nil {
							m.Databases = make(map[string]Database)
						}
						if _, exists := m.Databases[dbName]; !exists {
							m.Databases[dbName] = Database{}
						}
					} else {
						diags = append(diags, simpleDiag("project.unknown_section", "DPM0001", "", fmt.Sprintf("unknown section: %s", body)))
					}
				} else {
					diags = append(diags, simpleDiag("project.unknown_section", "DPM0001", "", fmt.Sprintf("unknown section: %s", body)))
				}
			}
			continue
		}
		// key = value
		k, v, ok := strings.Cut(trim, "=")
		if !ok {
			diags = append(diags, simpleDiag("project.syntax", "DPM0001", "", fmt.Sprintf("invalid line %d: %s", s.line, trim)))
			continue
		}
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		switch s.section {
		case "package":
			switch key {
			case "name":
				m.Package.Name = parseString(val)
			case "version":
				m.Package.Version = parseString(val)
			case "edition":
				m.Package.Edition = parseString(val)
			case "entry":
				m.Package.Entry = parseString(val)
			case "roots":
				m.Package.Roots, _ = parseStringArray(val)
			default:
				diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("unknown key: package.%s", key)))
			}
		case "build":
			switch key {
			case "mode":
				m.Build.Mode = parseString(val)
			case "out_dir":
				m.Build.OutDir = parseString(val)
			default:
				diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("unknown key: build.%s", key)))
			}
		case "target":
			switch key {
			case "triple":
				m.Target.Triple = parseString(val)
			default:
				diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("unknown key: target.%s", key)))
			}
		case "diagnostics":
			switch key {
			case "error_format":
				m.Diagnostics.ErrorFormat = parseString(val)
			case "color":
				m.Diagnostics.Color = parseString(val)
			case "max_errors":
				m.Diagnostics.MaxErrors = parseString(val)
			default:
				diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("unknown key: diagnostics.%s", key)))
			}
		case "ffi":
			switch key {
			case "libs":
				m.FFI.Libs, _ = parseStringArray(val)
			case "search":
				m.FFI.Search, _ = parseStringArray(val)
			default:
				diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("unknown key: ffi.%s", key)))
			}
		case "ffi.extern":
			ex := Extern{}
			switch key {
			case "name":
				ex.Name = parseString(val)
			case "lib":
				ex.Lib = parseString(val)
			default:
				diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("unknown key: ffi.extern.%s", key)))
			}
			// Append only if name set; otherwise defer until lib assigned on another line
			if ex.Name != "" || ex.Lib != "" {
				// Merge with last if present and partially filled
				if len(m.FFI.Externs) > 0 {
					last := &m.FFI.Externs[len(m.FFI.Externs)-1]
					if last.Name == "" && ex.Name != "" {
						last.Name = ex.Name
						continue
					}
					if last.Lib == "" && ex.Lib != "" {
						last.Lib = ex.Lib
						continue
					}
				}
				m.FFI.Externs = append(m.FFI.Externs, ex)
			}
		case "database":
			// Route key-value to either m.Database (default) or m.Databases[name]
			setDBField := func(setter func(db *Database)) {
				if s.dbName == "" {
					setter(&m.Database)
				} else {
					entry := m.Databases[s.dbName]
					setter(&entry)
					m.Databases[s.dbName] = entry
				}
			}
			switch key {
			case "engine":
				setDBField(func(db *Database) { db.Engine = parseString(val) })
			case "schema_only":
				setDBField(func(db *Database) { db.SchemaOnly = parseString(val) == "true" })
			case "debug_queries":
				setDBField(func(db *Database) { db.DebugQueries = parseString(val) == "true" })
			case "host":
				setDBField(func(db *Database) { db.Host = parseString(val) })
			case "port":
				setDBField(func(db *Database) { db.Port = parseString(val) })
			case "name":
				setDBField(func(db *Database) { db.Name = parseString(val) })
			case "user":
				setDBField(func(db *Database) { db.User = parseString(val) })
			case "password":
				setDBField(func(db *Database) { db.Password = parseString(val) })
			case "ssl_mode":
				setDBField(func(db *Database) { db.SslMode = parseString(val) })
			case "charset":
				setDBField(func(db *Database) { db.Charset = parseString(val) })
			case "timezone":
				setDBField(func(db *Database) { db.Timezone = parseString(val) })
			case "prefix":
				setDBField(func(db *Database) { db.Prefix = parseString(val) })
			case "max_conns":
				setDBField(func(db *Database) { db.MaxConns = parseString(val) })
			case "conn_timeout":
				setDBField(func(db *Database) { db.ConnTimeout = parseString(val) })
			case "options":
				setDBField(func(db *Database) { db.Options = parseString(val) })
			default:
				secName := "database"
				if s.dbName != "" {
					secName = "database." + s.dbName
				}
				diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("unknown key: %s.%s", secName, key)))
			}
		default:
			// outside any known section
			diags = append(diags, simpleDiag("project.unknown_key", "DPM0001", "", fmt.Sprintf("key outside a section: %s", key)))
		}
	}
	// Validate externs
	for _, ex := range m.FFI.Externs {
		if ex.Name == "" || ex.Lib == "" {
			diags = append(diags, simpleDiag("project.bad_extern", "DPM0001", "", "ffi.extern requires name and lib"))
		}
	}
	return m, diags
}

func parseString(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) && len(v) >= 2 {
		out := strings.Trim(v, `"`)
		out = strings.ReplaceAll(out, `\"`, `"`)
		return out
	}
	// allow bare words for enums (e.g., color=auto)
	if v != "" && !strings.ContainsAny(v, "[]") {
		return v
	}
	return ""
}

func parseStringArray(v string) ([]string, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, false
	}
	body := strings.TrimSpace(v[1 : len(v)-1])
	if body == "" {
		return []string{}, true
	}
	parts := splitCSV(body)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := parseString(strings.TrimSpace(p))
		if s == "" {
			// invalid element type
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func splitCSV(s string) []string {
	var out []string
	cur := strings.Builder{}
	inStr := false
	esc := false
	for _, r := range s {
		switch {
		case inStr:
			if esc {
				cur.WriteRune(r)
				esc = false
				continue
			}
			if r == '\\' {
				esc = true
				continue
			}
			if r == '"' {
				inStr = false
				continue
			}
			cur.WriteRune(r)
		default:
			if r == '"' {
				inStr = true
				continue
			}
			if r == ',' {
				out = append(out, cur.String())
				cur.Reset()
				continue
			}
			cur.WriteRune(r)
		}
	}
	out = append(out, cur.String())
	return out
}

func simpleDiag(path, id, file, msg string) diag.Diagnostic {
	return diag.Diagnostic{
		CodeID:  id,
		Domain:  "project",
		Title:   strings.TrimPrefix(path, "project."),
		Message: msg,
		Primary: diag.Label{Span: diag.Span{File: file}},
	}
}
