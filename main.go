package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/nfnt/resize"
	"html/template"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

const gonagallConfigFile = "gonagallconfig.json"

var gonagallConfig struct {
	BaseDir  string
	CacheDir string
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

type dummy struct {
	Subs []string
	Jpgs []string
}

func BrowseDirectory(w http.ResponseWriter, r *http.Request) {
	l := len("/")
	upath := r.URL.Path[l:]
	t, err := template.ParseFiles("template.browse.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	entries, err := ioutil.ReadDir(gonagallConfig.BaseDir + "/" + upath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var d dummy
	for _, r := range entries {
		if r.IsDir() && !strings.HasPrefix(r.Name(), ".") {
			d.Subs = append(d.Subs, upath+"/"+r.Name())
		} else if !r.IsDir() && strings.HasSuffix(r.Name(), ".jpg") {
			d.Jpgs = append(d.Jpgs, upath+"/"+r.Name())
		}
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

	fmt.Println("Starting to serve", fullPath, "with width", maxDim, "==> hashPath")

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

	resized := resize.Resize(maxDim, 0, origImage, resize.NearestNeighbor)
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
	fmt.Println("caching", fullPath, "to", hashPath, "size=", b.Len(), ratio)
	b.WriteTo(cacheFile)
}

func ServeThumb(w http.ResponseWriter, r *http.Request) {
	l := len("/thumb/")
	serveResizedImage(w, r.URL.Path[l:], 100)
}

func ServeSmall(w http.ResponseWriter, r *http.Request) {
	l := len("/view/")
	serveResizedImage(w, r.URL.Path[l:], 480)
}

func main() {
	readConfig()
	fmt.Println(gonagallConfig)
	http.HandleFunc("/", BrowseDirectory)
	http.HandleFunc("/thumb/", ServeThumb)
	http.HandleFunc("/view/", ServeSmall)
	//	http.HandleFunc("/original/", ServeOriginal)
	http.ListenAndServe(":8781", nil)
}
