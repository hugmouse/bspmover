package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/galaco/bsp"
	"log"
	"os"
	"strings"
	"time"
)

var (
	// Downloads folder
	downloads = os.Getenv("BSPMW_DOWNLOADS")
	tf2path   = os.Getenv("BSPMW_TF")
)

func main() {
	if downloads == "" {
		log.Fatal("[Error] BSPMW_DOWNLOADS is not set. You must set this environment variable to the path of your downloads folder.")
	}
	log.Println("[Info] Downloads folder:", downloads)

	// Validate path
	if _, err := os.Stat(downloads); os.IsNotExist(err) {
		log.Fatal("[Error] BSPMW_DOWNLOADS is not set to a valid folder. Please set this environment variable to the path of your downloads folder.")
	}

	if tf2path == "" {
		log.Fatal("[Error] BSPMW_TF is not set. You must set this environment variable to the path of your Team Fortress 2 installation.")
	}
	log.Println("[Info] TF2 folder:", tf2path)

	// Validate path
	if _, err := os.Stat(tf2path); os.IsNotExist(err) {
		log.Fatal("[Error] BSPMW_TF is not set to a valid folder. Please set this environment variable to the path of your downloads folder.")
	}

	// Create new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(watcher)

	// Start listening for events
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) {
					if event.Name[len(event.Name)-4:] == ".bsp" {
						mapNameWithExt := strings.Split(event.Name, "\\")[len(strings.Split(event.Name, "\\"))-1]
						log.Println("Map file created! Waiting a few seconds before moving it to the TF2 maps folder...")

						// Create a new _bsp reader
						_bsp, err := bsp.ReadFromFile(event.Name)
						if err != nil {
							log.Fatal(err)
						}

						if _bsp.Header().Version != 20 {
							log.Println("[Warn] This map is not compatible with Team Fortress 2. Ignoring. Version: ", _bsp.Header().Version)
							continue
						}

						time.Sleep(2 * time.Second)

						err = os.Rename(event.Name, tf2path+"\\"+mapNameWithExt)
						if err != nil {
							log.Fatal("An error occurred while moving the map file to the TF2 maps folder: ", err)
						}

						log.Println("Map file moved to TF2 maps folder!")
						log.Printf("You can start the map by typing 'map %s' in the console.\n", mapNameWithExt[:len(mapNameWithExt)-4])
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(downloads)
	if err != nil {
		log.Fatal(err)
	}

	// Block main goroutine forever
	<-make(chan struct{})
}
