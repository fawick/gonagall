package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/nfnt/resize"
	"html/template"
	"image"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

const gonagallConfigFile = "gonagallconfig.json"

var gonagallConfig struct {
	BaseDir   string
	CacheDir  string
	ThumbSize uint
	ViewSize  uint
	CatchAll  bool
}

func writeConfig() error {
	j, err := json.MarshalIndent(&gonagallConfig, "", "\t")
	if err != nil {
		fmt.Println("Error in writeConfig:", err)
		return err
	}
	outFile, err := os.Create(gonagallConfigFile)
	b := bytes.NewBuffer(j)
	if _, err := b.WriteTo(outFile); err != nil {
		fmt.Println("Error in writeConfig:", err)
		return err
	}
	return nil
}

func readConfig() error {
	s, _ := os.Getwd()
	fmt.Println("pwd", s)
	gonagallConfig.BaseDir = "."
	gonagallConfig.CacheDir = "/tmp"
	gonagallConfig.ThumbSize = 100
	gonagallConfig.ViewSize = 480
	inFile, err := os.Open(gonagallConfigFile)
	if err != nil {
		fmt.Println("Error in readConfig:", err)
		return writeConfig()
	}
	dec := json.NewDecoder(inFile)
	err = dec.Decode(&gonagallConfig)
	if err != nil {
		fmt.Println("Error in readConfig:", err)
		return err
	}
	return writeConfig()
}

type dirContents struct {
	Path     string
	SubDirs  []string
	JpgFiles []string
}

func scanDir(path string) (d dirContents, err error) {
	entries, err := ioutil.ReadDir(gonagallConfig.BaseDir + "/" + path)
	if err != nil {
		return
	}
	for _, r := range entries {
		n := strings.ToUpper(r.Name())
		if r.IsDir() {
			if !strings.HasPrefix(n, ".") {
				d.SubDirs = append(d.SubDirs, r.Name())
			}
		} else if strings.HasSuffix(n, ".JPG") || strings.HasSuffix(n, ".JPEG") {
			d.JpgFiles = append(d.JpgFiles, r.Name())
		}
	}
	d.Path = path
	return
}

func BrowseDirectory(w http.ResponseWriter, r *http.Request) {
	upath := mux.Vars(r)["directory"]
	t, err := template.ParseFiles("template.browse.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d, err := scanDir(upath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = t.Execute(w, d)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func serveResizedImage(w http.ResponseWriter, path string, maxDim uint) {
	fullPath := gonagallConfig.BaseDir + "/" + path
	h := sha1.New()
	if _, err := h.Write([]byte(gonagallConfig.CacheDir + "/" + fullPath + fmt.Sprint(maxDim))); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	hashPath := gonagallConfig.CacheDir + "/" + fmt.Sprintf("%x.jpg", h.Sum(nil))

	fmt.Println("Starting to serve", fullPath, "with width", maxDim, "==>", hashPath)

	if _, err := os.Stat(hashPath); err == nil {
		fmt.Println("Serving existing resized file:", fullPath, "with width", maxDim, "==> hashPath")
		b, err := ioutil.ReadFile(hashPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write(b)
		return
	} else if !os.IsNotExist(err) {
		http.Error(w, err.Error(), 500)
		return
	}

	origFile, _ := os.Open(fullPath)
	origImage, _ := jpeg.Decode(origFile)
	origFileStat, _ := origFile.Stat()
	origFile.Close()

	fmt.Println("decoded", fullPath)

	var resized image.Image
	//p := origImage.Bounds().Size()
	//if p.X > p.Y {
	//	resized = resize.Resize(maxDim, 0, origImage, resize.NearestNeighbor)
	//} else {
	resized = resize.Resize(0, maxDim, origImage, resize.NearestNeighbor)
	//}
	b := new(bytes.Buffer)
	jpeg.Encode(b, resized, nil)
	ratio := float64(b.Len()) / float64(origFileStat.Size()) * 100.0
	fmt.Println("re-encoded", fullPath, "to", hashPath, "size=", b.Len(), ratio)
	w.Write(b.Bytes())

	// cache the contents
	cacheFile, err := os.Create(hashPath)
	defer cacheFile.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	b.WriteTo(cacheFile)
}

func relativePath(r *http.Request) string {
	m := mux.Vars(r)
	var s string
	if d, ok := m["directory"]; ok {
		s += d + "/"
	}
	s += m["imagefile"]
	return s
}

func ServeThumb(w http.ResponseWriter, r *http.Request) {
	p := relativePath(r)
	serveResizedImage(w, p, gonagallConfig.ThumbSize)
}

func ServeSmall(w http.ResponseWriter, r *http.Request) {
	p := relativePath(r)
	serveResizedImage(w, p, gonagallConfig.ViewSize)
}

func ServeFull(w http.ResponseWriter, r *http.Request) {
	p := relativePath(r)
	http.ServeFile(w, r, gonagallConfig.BaseDir+"/"+p)
}

func ViewImage(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("template.view.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var d struct {
		Path, File string
	}
	m := mux.Vars(r)
	d.Path = m["directory"]
	d.File = m["imagefile"]

	err = t.Execute(w, d)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func imageSubrouters(r *mux.Route, f func(http.ResponseWriter, *http.Request)) {
	s := r.Subrouter()
	s.HandleFunc("/{imagefile}", f)
	s.HandleFunc("/{directory:[A-Za-z0-9/\\-_ ]+}/{imagefile}", f)
}

func main() {
	readConfig()
	os.Mkdir(gonagallConfig.CacheDir, os.ModePerm)
	r := mux.NewRouter()
	imageSubrouters(r.PathPrefix("/thumb"), ServeThumb)
	imageSubrouters(r.PathPrefix("/small"), ServeSmall)
	imageSubrouters(r.PathPrefix("/original"), ServeFull)
	imageSubrouters(r.PathPrefix("/view"), ViewImage)

	r.Path("/gallery/{directory:[A-Za-z0-9/\\-_ ]*}").HandlerFunc(BrowseDirectory)

	r.PathPrefix("/static").Handler(http.StripPrefix("/static", http.FileServer(http.Dir("static"))))

	if gonagallConfig.CatchAll {
		r.NotFoundHandler = http.RedirectHandler("/gallery", 301)
	}
	http.Handle("/", r)
	http.ListenAndServe(":8781", nil)
}
