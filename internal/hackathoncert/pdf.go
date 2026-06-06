package hackathoncert

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

// GenerateSimplePDF builds a minimal certificate PDF for hackathon participants/winners.
func GenerateSimplePDF(studentName, hackathonTitle, organizerLabel, certLabel string) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 24)
	pdf.CellFormat(0, 20, "Steppy Marketplace", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 14)
	pdf.CellFormat(0, 12, certLabel, "", 1, "C", false, 0, "")
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 14, studentName, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(0, 10, fmt.Sprintf("Hackathon: %s", hackathonTitle), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 10, fmt.Sprintf("Organizer: %s", organizerLabel), "", 1, "C", false, 0, "")
	pdf.Ln(20)
	pdf.SetFont("Arial", "I", 10)
	pdf.CellFormat(0, 8, "This certificate was issued by Steppy Marketplace.", "", 1, "C", false, 0, "")
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
