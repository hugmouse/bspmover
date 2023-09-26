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

func main() {
	downloads, tf2path := initializePaths()
	watcher := createWatcher(downloads)

	processingFiles := make(map[string]bool)
	startFSEventListener(watcher, processingFiles, downloads, tf2path)

	select {}
}

func initializePaths() (string, string) {
	downloads, err := getDownloadsFolder()
	if err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stat(downloads); os.IsNotExist(err) {
		log.Println("[Error] Cannot find Downloads folder!")
	}

	log.Println("[Info] Downloads folder:", downloads)

	tf2path, err := getTF2Path()
	if err != nil {
		log.Fatal("[Error] Failed to auto-detect Team Fortress 2 installation path:", err)
	}

	tf2path = path.Join(tf2path, "steamapps/common/Team Fortress 2/tf/maps")
	log.Println("[Info] TF2 folder:", tf2path)

	if _, err := os.Stat(tf2path); os.IsNotExist(err) {
		log.Fatal("[Error] Auto-detected TF2 path is not valid. Please set BSPMW_TF environment variable to the correct TF2 folder.")
	}

	return downloads, tf2path
}

func createWatcher(path string) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("[Info] Created new watcher.")
	return watcher
}

func startFSEventListener(watcher *fsnotify.Watcher, processingFiles map[string]bool, downloads, tf2path string) {
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) && strings.HasSuffix(event.Name, ".bsp") {
					processMapFile(event.Name, processingFiles, tf2path)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err := watcher.Add(downloads)
	if err != nil {
		log.Fatal(err)
	}
}

func processMapFile(fileName string, processingFiles map[string]bool, tf2path string) {
	mapNameWithExt := strings.Split(fileName, "\\")[len(strings.Split(fileName, "\\"))-1]

	if processingFiles[fileName] {
		log.Printf("File %s is already being processed, skipping.", fileName)
		delete(processingFiles, fileName)
		return
	}

	processingFiles[fileName] = true
	log.Println("Map file created! Waiting for it to be fully downloaded before moving it to the TF2 maps folder...")

	stableDuration := 2 * time.Second
	ticker := time.NewTicker(stableDuration)
	var lastSize int64

	for {
		select {
		case <-ticker.C:
			fileInfo, err := os.Stat(fileName)
			if err != nil {
				log.Println("Error while checking file size:", err)
				return
			}

			// Firefox might create a 0 byte file before the actual file is downloaded
			if fileInfo.Size() == 0 {
				return
			}

			if fileInfo.Size() == lastSize {
				ticker.Stop()
				goto ContinueMoving
			}

			lastSize = fileInfo.Size()
		}
	}

ContinueMoving:
	_bsp, err := bsp.ReadFromFile(fileName)
	if err != nil {
		log.Printf("Error while reading BSP %q: %s\n", fileName, err)
		return
	}

	if _bsp.Header().Version != 20 {
		log.Println("[Warn] This map is not compatible with Team Fortress 2. Ignoring. Version: ", _bsp.Header().Version)
		return
	}

	err = os.Rename(fileName, tf2path+"\\"+mapNameWithExt)
	if err != nil {
		log.Fatal("An error occurred while moving the map file to the TF2 maps folder: ", err)
	}

	log.Println("Map file moved to TF2 maps folder!")
	log.Printf("You can start the map by typing 'map %s' in the console.\n", mapNameWithExt[:len(mapNameWithExt)-4])
}

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
