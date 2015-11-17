package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
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
	gonagallConfig.CacheDir = "/tmp/gonagall"
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
	Path       string
	SubDirs    []string
	ImageFiles []string
}

type breadCrumb struct {
	Text string
	Link string
}

func (d dirContents) Breadcrumbs() []breadCrumb {
	var v []breadCrumb
	var l string
	for _, s := range strings.Split(d.Path, "/") {
		if s == "" {
			continue
		}
		l += "/" + s
		v = append(v, breadCrumb{s, l})
	}
	return v
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
			d.ImageFiles = append(d.ImageFiles, r.Name())
		} else if strings.HasSuffix(n, ".TIF") || strings.HasSuffix(n, ".TIFF") {
			d.ImageFiles = append(d.ImageFiles, r.Name())
		}
	}
	d.Path = path
	return
}

func BrowseDirectory(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
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

func resizeImage(origName, newName string, maxDim uint, square bool) error {
	var args []string
	if square {
		args = append(args,
			"-define", fmt.Sprintf("jpeg:size=%dx%d", maxDim*2, maxDim*2),
			"-resize", fmt.Sprintf("%dx%d^", maxDim, maxDim),
			"-gravity", "center",
			"-extent", fmt.Sprintf("%dx%d^", maxDim, maxDim),
		)
	} else {
		args = append(args,
			"-define", fmt.Sprintf("jpeg:size=%dx%d", maxDim*2, maxDim*2),
			"-resize", fmt.Sprintf("%dx%d>", maxDim, maxDim),
		)
	}
	args = append(args, origName, newName)
	cmd := exec.Command("convert", args...)
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func serveResizedImage(w http.ResponseWriter, r *http.Request, maxDim uint, square bool) {
	fullPath := gonagallConfig.BaseDir + "/" + r.URL.Path
	h := sha1.New()
	if _, err := h.Write([]byte(gonagallConfig.CacheDir + "/" + fullPath + fmt.Sprint(maxDim))); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	hashPath := gonagallConfig.CacheDir + "/" + fmt.Sprintf("%x.jpg", h.Sum(nil))

	fmt.Println("Starting to serve", fullPath, "with width", maxDim, "==>", hashPath)

	if _, err := os.Stat(hashPath); err != nil {
		if !os.IsNotExist(err) {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Println("Converting file:", fullPath, "with width", maxDim, "==>", hashPath)
		if square {
			resizeImage(fullPath, hashPath, maxDim, true)
		} else {
			resizeImage(fullPath, hashPath, maxDim, false)
		}
	}
	http.ServeFile(w, r, hashPath)
}

func setupHandler(prefix string, f http.HandlerFunc) {
	http.Handle(prefix+"/", http.StripPrefix(prefix, http.HandlerFunc(f)))
}

func main() {
	readConfig()
	os.Mkdir(gonagallConfig.CacheDir, os.ModePerm)
	setupHandler("/gallery", BrowseDirectory)
	setupHandler("/thumb", func(w http.ResponseWriter, r *http.Request) {
		serveResizedImage(w, r, gonagallConfig.ThumbSize, true)
	})
	setupHandler("/small", func(w http.ResponseWriter, r *http.Request) {
		serveResizedImage(w, r, gonagallConfig.ViewSize, true)
	})
	setupHandler("/original", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, gonagallConfig.BaseDir+"/"+r.URL.Path)
	})
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
	if gonagallConfig.CatchAll {
		http.Handle("/", http.RedirectHandler("/gallery/", 301))
	}
	http.ListenAndServe(":8782", nil)
}
