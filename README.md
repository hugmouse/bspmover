![README Banner](https://user-images.githubusercontent.com/44648612/230716976-54db18d9-0ec2-4f31-b7f3-fc1e9fbc4a76.png)

# BSP Mover

This is a simple tool to move BSP files from one directory to another.

It adds file watcher to downloads folder and when browser finishes downloading a TF2 map it moves it to the maps folder.

## Usage

- Set environment variables. 
  - `BSPMW_DOWNLOADS` - path to the downloads folder
  - `BSPMW_TF` - path to the TF2 folder
- Run `bsp-mover.exe`

## Building

- Install Go
- Run `go get ./...` to install dependencies
- Run `go build` to build the project