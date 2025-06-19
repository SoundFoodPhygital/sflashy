// sflashy - A simple utility to flash an image to a device, written in Go.
// author: Gemini (re-implementation of script by Matteo Spanio)
// date: 2025-06-19
// license: GPL3
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/jaypipes/ghw"
)

// (I codici colore e le altre funzioni come usage() e listBlockDevices() rimangono invariate)
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

// progressWriter rimane invariato
type progressWriter struct {
	total     int64
	out       io.Writer // Scriviamo il progresso su un output generico
	lastShown int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.total += int64(n)
	if pw.total-pw.lastShown > 2*1024*1024 {
		// Scrive il progresso sull'output specificato (es. os.Stdout)
		fmt.Fprintf(pw.out, "\r%sWriting... %.2f GB copied%s", ColorYellow, float64(pw.total)/(1024*1024*1024), ColorReset)
		pw.lastShown = pw.total
	}
	return n, nil
}

// flashDevice ora accetta interfacce, rendendola testabile.
// source: Lo stream di dati dell'immagine.
// dest: Lo stream di dati del dispositivo di destinazione.
// userInput: Lo stream per leggere l'input dell'utente (la conferma 'y/N').
// termOut: Lo stream per scrivere i messaggi all'utente.
func flashDevice(source io.Reader, dest io.Writer, userInput io.Reader, termOut io.Writer) error {
	fmt.Fprintln(termOut, "Flashing image to device. This will erase all data on the device.")
	fmt.Fprint(termOut, "Are you sure? [y/N]: ")

	reader := bufio.NewReader(userInput)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if response != "y" && response != "Y" {
		fmt.Fprintln(termOut, "Operation cancelled.")
		return nil
	}

	fmt.Fprintln(termOut, "Starting flash operation...")

	pw := &progressWriter{out: termOut}
	readerWithProgress := io.TeeReader(source, pw)

	// Usiamo io.CopyBuffer per un maggiore controllo e potenziale efficienza
	buf := make([]byte, 32*1024*1024) // Buffer da 32MB come in dd bs=32M
	_, err := io.CopyBuffer(dest, readerWithProgress, buf)

	if err != nil {
		fmt.Fprintln(termOut) // Nuova riga per non sovrascrivere il progresso
		return fmt.Errorf("error while writing to device: %w", err)
	}

	// La chiamata a Sync() deve essere fatta sul file reale, non sull'interfaccia.
	// La gestiamo nel chiamante (la funzione main).

	fmt.Fprintln(termOut) // Nuova riga finale
	fmt.Fprintln(termOut, ColorGreen+"\nFlash completed successfully!"+ColorReset)
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
	devicePath := args[2]

	// Check if the image file exists and is a regular file
	info, err := os.Stat(imageFile)
	if os.IsNotExist(err) {
		log.Fatalf(ColorRed+"Error: Image file not found: %s"+ColorReset, imageFile)
	}
	if info.IsDir() {
		log.Fatalf(ColorRed+"Error: The provided image path is a directory, not a file: %s"+ColorReset, imageFile)
	}

	// Check if the device exists and is a block device
	info, err = os.Stat(devicePath)
	if os.IsNotExist(err) {
		log.Fatalf(ColorRed+"Error: Device not found: %s"+ColorReset, devicePath)
	}
	// os.ModeDevice indicates it's a device file (/dev/...).
	// We check that this bit is set in the file mode.
	if (info.Mode() & os.ModeDevice) == 0 {
		log.Fatalf(ColorRed+"Error: The provided path is not a block device: %s"+ColorReset, devicePath)
	}

	// --- Logica di esecuzione ---

	// Apriamo i file/device reali qui
	source, err := os.Open(imageFile)
	if err != nil {
		log.Fatalf(ColorRed+"Error: Could not open image file %s: %v"+ColorReset, imageFile, err)
	}
	defer source.Close()

	dest, err := os.OpenFile(devicePath, os.O_WRONLY|os.O_EXCL, 0666)
	if err != nil {
		log.Fatalf(ColorRed+"Error: Could not open device %s for writing: %v"+ColorReset, devicePath, err)
	}
	defer dest.Close()

	// Eseguiamo la logica passando gli stream reali
	err = flashDevice(source, dest, os.Stdin, os.Stdout)
	if err != nil {
		log.Fatalf(ColorRed+"\nAn error occurred: %v"+ColorReset, err)
	}

	// Eseguiamo Sync sul file descriptor reale dopo che flashDevice ha terminato
	fmt.Println("Finalizing write (syncing)...")
	if err := dest.Sync(); err != nil {
		log.Fatalf(ColorRed+"Failed to sync data to device: %v"+ColorReset, err)
	}
}
