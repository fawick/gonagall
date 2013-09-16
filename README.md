gonagall
========

Web-gallery system written in Go Language

Installation
-----------

	go get github.com/fawick/gonagall

On first run, gonagall will create a gonagallconfig.json with the default settings. Edit this file
to point to your picture base folder and restart gonagall. Point your browser to localhost:8080 and
browse through your pictures.

Caching
-----------

When a directory is accessed for the first time, gonagall creates thumbnail files for each image in
the directory. The thumbnails are cached to disk, change the value CacheDir in gonacallconfig.json for
setting the path for this.

Analogously, individual image files are resized and cached as well.

TODO
----

- Background service for resizing images

