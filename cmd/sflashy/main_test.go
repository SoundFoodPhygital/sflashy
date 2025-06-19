package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestFlashDeviceSuccess verifica il caso in cui l'utente conferma l'operazione.
func TestFlashDeviceSuccess(t *testing.T) {
	// 1. Setup: Creiamo i nostri "fake" streams
	sourceData := "Questa è l'immagine di test"
	source := strings.NewReader(sourceData) // Fake immagine sorgente

	var dest bytes.Buffer // Fake dispositivo di destinazione (un buffer in memoria)

	userInput := strings.NewReader("y\n") // Fake input utente che scrive 'y' e preme invio

	var termOut bytes.Buffer // Fake terminale per catturare l'output

	// 2. Esecuzione: Chiamiamo la funzione da testare con i nostri fake
	err := flashDevice(source, &dest, userInput, &termOut)

	// 3. Asserzioni: Verifichiamo che tutto sia andato come previsto
	if err != nil {
		t.Errorf("flashDevice ha restituito un errore inaspettato: %v", err)
	}

	// Controlliamo che i dati scritti sul "dispositivo" siano corretti
	if dest.String() != sourceData {
		t.Errorf("I dati scritti non corrispondono alla sorgente. Got: %q, Want: %q", dest.String(), sourceData)
	}

	// Controlliamo che il messaggio di successo sia stato stampato
	output := termOut.String()
	if !strings.Contains(output, "Flash completed successfully!") {
		t.Errorf("L'output non contiene il messaggio di successo. Got: %q", output)
	}
}

// TestFlashDeviceCancel verifica il caso in cui l'utente annulla l'operazione.
func TestFlashDeviceCancel(t *testing.T) {
	// 1. Setup
	source := strings.NewReader("Dati che non dovrebbero mai essere scritti")
	var dest bytes.Buffer
	userInput := strings.NewReader("n\n") // L'utente scrive 'n'
	var termOut bytes.Buffer

	// 2. Esecuzione
	err := flashDevice(source, &dest, userInput, &termOut)

	// 3. Asserzioni
	if err != nil {
		t.Errorf("flashDevice ha restituito un errore inaspettato in caso di annullamento: %v", err)
	}

	// La cosa più importante: il buffer di destinazione deve essere vuoto!
	if dest.Len() > 0 {
		t.Errorf("Sono stati scritti dei dati anche se l'operazione è stata annullata. Bytes scritti: %d", dest.Len())
	}

	// Controlliamo che il messaggio di annullamento sia stato stampato
	output := termOut.String()
	if !strings.Contains(output, "Operation cancelled.") {
		t.Errorf("L'output non contiene il messaggio di annullamento. Got: %q", output)
	}
}

// TestProgressWriter verifica che il contatore di progresso funzioni correttamente.
func TestProgressWriter(t *testing.T) {
	// Setup
	var capturedOutput bytes.Buffer
	pw := &progressWriter{out: &capturedOutput}

	// Esecuzione
	testData := make([]byte, 1024) // 1KB di dati
	n, err := pw.Write(testData)
	if err != nil {
		t.Fatalf("pw.Write ha restituito un errore: %v", err)
	}

	// Asserzioni
	if n != 1024 {
		t.Errorf("Write ha restituito un numero di byte errato. Got: %d, Want: %d", n, 1024)
	}

	if pw.total != 1024 {
		t.Errorf("Il totale dei byte non è stato aggiornato correttamente. Got: %d, Want: %d", pw.total, 1024)
	}

	// Scriviamo abbastanza dati da triggerare la stampa del progresso
	largeData := make([]byte, 3*1024*1024) // 3MB
	_, _ = pw.Write(largeData)

	if !strings.Contains(capturedOutput.String(), "Writing...") {
		t.Error("Il messaggio di progresso non è stato scritto sull'output")
	}
}
