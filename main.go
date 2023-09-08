package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/galaco/bsp"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func getDownloadsFolder() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	downloadsPath := filepath.Join(currentUser.HomeDir, "Downloads")

	return downloadsPath, nil
}

func getTF2Path() (string, error) {
	var subkey = `SOFTWARE\Valve\Steam`

	k, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return "", err
	}

	var handle syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, k, 0, syscall.KEY_READ, &handle); err != nil {
		return "", err
	}
	defer syscall.RegCloseKey(handle)

	var buf [1024]uint16
	n := uint32(len(buf))
	if err := syscall.RegQueryValueEx(handle, syscall.StringToUTF16Ptr("SteamPath"), nil, nil, (*byte)(unsafe.Pointer(&buf[0])), &n); err != nil {
		return "", err
	}

	return syscall.UTF16ToString(buf[:]), nil
}

func main() {
	downloads, err := getDownloadsFolder()
	if err != nil {
		log.Fatal(err)
	}
	// Validate path
	if _, err := os.Stat(downloads); os.IsNotExist(err) {
		log.Println("[Error] Cannot find Downloads folder!")
	}
	log.Println("[Info] Downloads folder:", downloads)

	// Try to auto-detect TF2 folder on Windows
	tf2path, err := getTF2Path()
	if err != nil {
		log.Fatal("[Error] Failed to auto-detect Team Fortress 2 installation path:", err)
	}

	tf2path = path.Join(tf2path, "steamapps/common/Team Fortress 2/tf/maps")
	log.Println("[Info] TF2 folder:", tf2path)

	// Validate path
	if _, err := os.Stat(tf2path); os.IsNotExist(err) {
		log.Fatal("[Error] Auto-detected TF2 path is not valid. Please set BSPMW_TF environment variable to the correct TF2 folder.")
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
	log.Println("[Info] Created new watcher.")

	// Create a map to track files being processed
	processingFiles := make(map[string]bool)

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

						// Check if the file is already being processed
						if processingFiles[event.Name] {
							log.Printf("File %s is already being processed, skipping.", event.Name)
							// Remove the file from the map, sometimes it happens with Firefox that it creates a file with the same name
							delete(processingFiles, event.Name)
							continue
						}

						// Mark the file as being processed
						processingFiles[event.Name] = true

						log.Println("Map file created! Waiting for it to be fully downloaded before moving it to the TF2 maps folder...")

						// Wait for the file to stabilize (no changes in size) for a certain duration
						stableDuration := 1 * time.Second
						ticker := time.NewTicker(stableDuration)
						var lastSize int64

						for {
							select {
							case <-ticker.C:
								fileInfo, err := os.Stat(event.Name)
								if err != nil {
									log.Println("Error while checking file size:", err)
									return
								}

								if fileInfo.Size() == lastSize {
									// File size hasn't changed, it's likely fully downloaded
									ticker.Stop()
									goto ContinueMoving
								}

								lastSize = fileInfo.Size()
							}
						}

					ContinueMoving:
						// Create a new _bsp reader
						_bsp, err := bsp.ReadFromFile(event.Name)
						if err != nil {
							log.Fatal(err)
						}

						if _bsp.Header().Version != 20 {
							log.Println("[Warn] This map is not compatible with Team Fortress 2. Ignoring. Version: ", _bsp.Header().Version)
							continue
						}

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
