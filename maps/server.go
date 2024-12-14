package maps

import (
	"archive/zip"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	lpath "path"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slices"

	"github.com/opennox/libs/ifs"
)

var _ http.Handler = (*Server)(nil)

const (
	contentTypeZIP = "application/zip"
)

var (
	allowedMapExt = []string{
		".map", ".rul", // original ones
		".go",         // Go map scripts
		".lua",        // LUA map scripts
		".txt", ".md", // text files (README, etc)
		// for future extensibility
		".json", ".yml", ".yaml", // metadata
		".png", ".jpg", // image files
		".mp3", ".ogg", // audio files
	}
	excludeMapExt = []string{
		".nxz",                // autogenerated
		".zip", ".tar", ".gz", // don't compress twice
	}
	allowedMapFiles = []string{
		"go.mod", "go.sum", // part of the dev environment for map scripts
		"LICENSE", // no extension to whitelist
	}
	excludeMapFiles = []string{
		"user.rul", // user defined, should not be distributed
		"temp.bmp", // temporary
	}
	lowerMapFileExt = []string{
		".map",
		".rul",
	}
)

// IsAllowedFile checks if the file with a given name is allowed to be distributed with the map.
func IsAllowedFile(path string) bool {
	if path == "" || path == "." {
		return true
	}
	if strings.HasPrefix(path, ".") && !strings.HasPrefix(path, "./") {
		return false
	}
	path = filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))
	if slices.Contains(lowerMapFileExt, ext) {
		path = strings.ToLower(path)
	}
	for _, name := range excludeMapFiles {
		if path == name {
			return false
		}
	}
	for _, name := range allowedMapFiles {
		if path == name {
			return true
		}
	}
	for _, e := range excludeMapExt {
		if e == ext {
			return false
		}
	}
	for _, e := range allowedMapExt {
		if e == ext {
			return true
		}
	}
	return false // unrecognized
}

// CompressMap collects and compresses relevant files from Nox/OpenNox map directory.
func CompressMap(w io.Writer, fss fs.FS, dir string) error {
	if fss == nil {
		fss = os.DirFS(dir)
		dir = "."
	}
	zw := zip.NewWriter(w)
	defer zw.Close()
	dir = lpath.Clean(dir)
	pref := strings.TrimSuffix(dir, "/") + "/"
	return fs.WalkDir(fss, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(path, pref)
		name = lpath.Clean(name)
		if name != "." && strings.HasPrefix(name, ".") {
			// Skip hidden files and folders.
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Type()&fs.ModeSymlink != 0 {
				return filepath.SkipDir // Always skip symlinks.
			}
			return nil // Continue into directories.
		}
		if !d.Type().IsRegular() {
			return nil // Skip symlinks and other non-regular files.
		}
		ext := strings.ToLower(lpath.Ext(name))
		if slices.Contains(lowerMapFileExt, ext) {
			name = lpath.Join(lpath.Dir(name), strings.ToLower(lpath.Base(name)))
		}
		if !IsAllowedFile(path) {
			return nil // skip
		}
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		r, err := fss.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(f, r)
		return err
	})
}

func NewServer(log *slog.Logger, path string) *Server {
	s := &Server{
		log:  log,
		path: path,
		mux:  httprouter.New(),
	}
	s.mux.Handle("HEAD", "/api/v0/maps/", s.handleMapList)
	s.mux.Handle("GET", "/api/v0/maps/", s.handleMapList)

	s.mux.Handle("HEAD", "/api/v0/maps/:map", s.handleMap)
	s.mux.Handle("GET", "/api/v0/maps/:map", s.handleMap)
	s.mux.Handle("GET", "/api/v0/maps/:map/download", s.handleMapDownload)
	return s
}

type Server struct {
	log  *slog.Logger
	mux  *httprouter.Router
	path string
}

func (s *Server) RegisterOnMux(mux *http.ServeMux) {
	mux.Handle("/api/v0/maps/", s)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.log.Debug("http request", "method", r.Method, "url", r.URL)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) serveJSON(w http.ResponseWriter, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	enc.Encode(obj)
}

func (s *Server) handleMapList(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	switch r.Method {
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	case "HEAD", "OPTIONS":
		w.WriteHeader(http.StatusOK)
	case "GET":
		list, err := Scan(s.log, s.path, nil)
		if err != nil {
			s.log.Error("error serving map list", "err", err)
			if len(list) == 0 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// serve at least some maps
		}
		s.serveJSON(w, list)
	}
}

func (s *Server) handleMap(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	name := p.ByName("map")
	if name == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	info, err := ReadMapInfo(filepath.Join(s.path, name))
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		s.log.Error("error serving map", "name", name, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.serveJSON(w, info)
}

func (s *Server) handleMapDownload(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	name := strings.ToLower(p.ByName("map"))
	if name == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	log := s.log.With("name", name)
	base := filepath.Join(s.path, name)
	base = ifs.Normalize(base)

	fname := name + ".map"
	fpath := filepath.Join(base, fname)
	fpath = ifs.Normalize(fpath)

	fi, err := os.Stat(fpath)
	if os.IsNotExist(err) || fi.IsDir() || fi.Size() == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		log.Error("error serving map", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	accept := r.Header.Get("Accept")
	if accept == "" {
		// serve the map file itself
		f, err := os.Open(fpath)
		if err != nil {
			log.Error("error serving map", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		http.ServeContent(w, r, fname, fi.ModTime(), f)
		return
	}
	// serve compressed map file
	w.Header().Set("Content-Type", contentTypeZIP)
	err = CompressMap(w, nil, base)
	if err != nil {
		log.Error("error serving map", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
