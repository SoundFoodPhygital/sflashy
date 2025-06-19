// sflashy - A simple utility to flash an image to a device, written in Go.
// author: Gemini (re-implementation of script by Matteo Spanio)
// date: 2025-06-19
// license: GPL3

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/jaypipes/ghw"
)

// ANSI color codes for better output
const (
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorReset  = "\033[0m"
)

// usage prints the help message, including available block devices.
func usage() {
	fmt.Println("Usage: flash <image-file> <device>")
	fmt.Println("Example: flash ~/Downloads/ubuntu.img /dev/sdb")
	fmt.Println("\nIf the device is mounted, please unmount it first.")
	fmt.Println("Example: umount /dev/sdb1")

	fmt.Println(ColorGreen + "\nAvailable devices:" + ColorReset)
	listBlockDevices()
}

// listBlockDevices prints a list of available block storage devices.
// It replaces the 'lsblk -p' command.
func listBlockDevices() {
	block, err := ghw.Block()
	if err != nil {
		log.Fatalf("Error getting block device info: %v", err)
	}

	fmt.Printf("%-15s %10s  %s\n", "NAME", "SIZE", "MODEL")
	fmt.Println(strings.Repeat("-", 40))

	for _, disk := range block.Disks {
		// ghw.Disk.SizeBytes is an uint64, we convert it to float64 for division
		sizeGB := float64(disk.SizeBytes) / (1024 * 1024 * 1024)
		fmt.Printf("%-15s %9.2f GB  %s\n", "/dev/"+disk.Name, sizeGB, disk.Model)
	}
}

// progressWriter is a helper to show progress during the copy operation.
// It implements the io.Writer interface.
type progressWriter struct {
	total     int64
	lastShown int64
}

// Write implements the io.Writer interface.
// It is called by io.Copy for each chunk of data written.
func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.total += int64(n)

	// Update progress every 2MB to avoid flooding the console
	if pw.total-pw.lastShown > 2*1024*1024 {
		// Carriage return '\r' moves the cursor to the beginning of the line
		fmt.Printf("\r%sWriting... %.2f GB copied%s", ColorYellow, float64(pw.total)/(1024*1024*1024), ColorReset)
		pw.lastShown = pw.total
	}

	return n, nil
}

// flashDevice writes the image file to the specified device.
// It replaces the 'dd' command.
func flashDevice(imagePath, devicePath string) error {
	// Open the source image file
	source, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("could not open image file %s: %w", imagePath, err)
	}
	defer source.Close()

	// Open the destination device for writing.
	// Use os.O_WRONLY for write-only and os.O_EXCL to ensure it's not a directory.
	dest, err := os.OpenFile(devicePath, os.O_WRONLY|os.O_EXCL, 0666)
	if err != nil {
		return fmt.Errorf("could not open device %s for writing: %w", devicePath, err)
	}
	defer dest.Close()

	fmt.Printf("Flashing %s to %s. This will erase all data on the device.\n", imagePath, devicePath)
	fmt.Print("Are you sure? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Println("Operation cancelled.")
		return nil // Not an error, user cancelled
	}

	fmt.Println("Starting flash operation...")

	// Create a progress writer and a TeeReader to write to both the device and the progress writer
	pw := &progressWriter{}
	readerWithProgress := io.TeeReader(source, pw)

	// io.Copy does the heavy lifting of reading from source and writing to dest.
	// It uses an internal buffer, which is efficient.
	_, err = io.Copy(dest, readerWithProgress)
	if err != nil {
		return fmt.Errorf("\nerror while writing to device: %w", err)
	}

	// This is the equivalent of dd's conv=fsync.
	// It ensures all buffered data is written to the underlying device.
	fmt.Println("\nFinalizing write (syncing)...")
	err = dest.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync data to device: %w", err)
	}

	fmt.Println(ColorGreen + "\nFlash completed successfully!" + ColorReset)
	return nil
}

func main() {
	// Configure logger to not print timestamps
	log.SetFlags(0)

	// --- Argument and Permission Checks ---

	args := os.Args
	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
		usage()
		os.Exit(0)
	}

	// Check for root privileges (EUID == 0 on Unix-like systems)
	if os.Geteuid() != 0 {
		log.Fatal(ColorRed + "Error: This program must be run as root." + ColorReset)
	}

	if len(args) != 3 {
		usage()
		os.Exit(1)
	}

	imageFile := args[1]
	device := args[2]

	// Check if the image file exists and is a regular file
	info, err := os.Stat(imageFile)
	if os.IsNotExist(err) {
		log.Fatalf(ColorRed+"Error: Image file not found: %s"+ColorReset, imageFile)
	}
	if info.IsDir() {
		log.Fatalf(ColorRed+"Error: The provided image path is a directory, not a file: %s"+ColorReset, imageFile)
	}

	// Check if the device exists and is a block device
	info, err = os.Stat(device)
	if os.IsNotExist(err) {
		log.Fatalf(ColorRed+"Error: Device not found: %s"+ColorReset, device)
	}
	// os.ModeDevice indicates it's a device file (/dev/...).
	// We check that this bit is set in the file mode.
	if (info.Mode() & os.ModeDevice) == 0 {
		log.Fatalf(ColorRed+"Error: The provided path is not a block device: %s"+ColorReset, device)
	}

	// --- Execute the core logic ---
	if err := flashDevice(imageFile, device); err != nil {
		log.Fatalf(ColorRed+"\nAn error occurred: %v"+ColorReset, err)
	}
}
