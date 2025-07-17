package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/ledongthuc/pdf"
	"github.com/russross/blackfriday/v2"
	"github.com/tealeg/xlsx"
	"github.com/unidoc/unioffice/document"
	"github.com/unidoc/unioffice/presentation"
	"gopkg.in/yaml.v2"
)

// Reader interface defines the contract for all file readers
type Reader interface {
	Parse(filePath string) (string, error)
}

// TXTReader handles plain text files
type TXTReader struct{}

func (r *TXTReader) Parse(filePath string) (string, error) {
	log.Printf("Reading TXT file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}
	return string(content), nil
}

// PDFReader handles PDF files
type PDFReader struct{}

func (r *PDFReader) Parse(filePath string) (string, error) {
	log.Printf("Reading PDF file from %s", filePath)
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening PDF: %w", err)
	}
	defer file.Close()

	var text strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		
		text.WriteString(fmt.Sprintf("Page %d\n", i))
		pageText, err := page.GetPlainText()
		if err != nil {
			log.Printf("Error reading page %d: %v", i, err)
			continue
		}
		text.WriteString(pageText)
		text.WriteString("\n")
	}
	return text.String(), nil
}

// DOCXReader handles Word documents
type DOCXReader struct{}

func (r *DOCXReader) Parse(filePath string) (string, error) {
	log.Printf("Reading DOCX file from %s", filePath)
	doc, err := document.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening DOCX: %w", err)
	}
	defer doc.Close()

	var text strings.Builder
	for i, para := range doc.Paragraphs() {
		text.WriteString(fmt.Sprintf("Paragraph %d:\n", i+1))
		for _, run := range para.Runs() {
			text.WriteString(run.Text())
		}
		text.WriteString("\n")
	}
	return text.String(), nil
}

// JSONReader handles JSON files
type JSONReader struct{}

func (r *JSONReader) Parse(filePath string) (string, error) {
	log.Printf("Reading JSON file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return "", fmt.Errorf("error parsing JSON: %w", err)
	}

	prettyJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return string(content), nil // fallback to raw content
	}
	return string(prettyJSON), nil
}

// JSONLReader handles JSON Lines files
type JSONLReader struct{}

func (r *JSONLReader) Parse(filePath string) (string, error) {
	log.Printf("Reading JSON Lines file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var result strings.Builder
	
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		var data interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			result.WriteString(fmt.Sprintf("Line %d (invalid JSON): %s\n", i+1, line))
			continue
		}
		
		prettyJSON, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			result.WriteString(fmt.Sprintf("Line %d: %s\n", i+1, line))
		} else {
			result.WriteString(fmt.Sprintf("Line %d:\n%s\n", i+1, string(prettyJSON)))
		}
	}
	return result.String(), nil
}

// XMLReader handles XML files
type XMLReader struct{}

func (r *XMLReader) Parse(filePath string) (string, error) {
	log.Printf("Reading XML file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Try to parse and pretty-print XML
	var data interface{}
	if err := xml.Unmarshal(content, &data); err != nil {
		// If parsing fails, return raw content
		return string(content), nil
	}

	prettyXML, err := xml.MarshalIndent(data, "", "  ")
	if err != nil {
		return string(content), nil
	}
	return string(prettyXML), nil
}

// YAMLReader handles YAML files
type YAMLReader struct{}

func (r *YAMLReader) Parse(filePath string) (string, error) {
	log.Printf("Reading YAML file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	var data interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return "", fmt.Errorf("error parsing YAML: %w", err)
	}

	prettyYAML, err := yaml.Marshal(data)
	if err != nil {
		return string(content), nil
	}
	return string(prettyYAML), nil
}

// HTMLReader handles HTML files
type HTMLReader struct{}

func (r *HTMLReader) Parse(filePath string) (string, error) {
	log.Printf("Reading HTML file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("error parsing HTML: %w", err)
	}

	return doc.Text(), nil
}

// MarkdownReader handles Markdown files
type MarkdownReader struct{}

func (r *MarkdownReader) Parse(filePath string) (string, error) {
	log.Printf("Reading Markdown file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Convert markdown to HTML, then extract text
	html := blackfriday.Run(content)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return string(content), nil // fallback to raw content
	}

	return doc.Text(), nil
}

// ExcelReader handles Excel files
type ExcelReader struct{}

func (r *ExcelReader) Parse(filePath string) (string, error) {
	log.Printf("Reading Excel file from %s", filePath)
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening Excel file: %w", err)
	}
	defer f.Close()

	var result strings.Builder
	for _, sheetName := range f.GetSheetList() {
		result.WriteString(fmt.Sprintf("Sheet Name: %s\n", sheetName))
		
		rows, err := f.GetRows(sheetName)
		if err != nil {
			log.Printf("Error reading sheet %s: %v", sheetName, err)
			continue
		}
		
		for _, row := range rows {
			result.WriteString(strings.Join(row, "\t"))
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}
	return result.String(), nil
}

// XLSReader handles older Excel files
type XLSReader struct{}

func (r *XLSReader) Parse(filePath string) (string, error) {
	log.Printf("Reading XLS file from %s", filePath)
	file, err := xlsx.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening XLS file: %w", err)
	}

	var result strings.Builder
	for _, sheet := range file.Sheets {
		result.WriteString(fmt.Sprintf("Sheet Name: %s\n", sheet.Name))
		
		for _, row := range sheet.Rows {
			var rowData []string
			for _, cell := range row.Cells {
				text := cell.String()
				rowData = append(rowData, text)
			}
			result.WriteString(strings.Join(rowData, "\t"))
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}
	return result.String(), nil
}

// PPTXReader handles PowerPoint files
type PPTXReader struct{}

func (r *PPTXReader) Parse(filePath string) (string, error) {
	log.Printf("Reading PowerPoint file from %s", filePath)
	pres, err := presentation.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening PowerPoint file: %w", err)
	}
	defer pres.Close()

	var result strings.Builder
	for i, slide := range pres.Slides() {
		result.WriteString(fmt.Sprintf("Slide %d:\n", i+1))
		
		for _, shape := range slide.Shapes() {
			if shape.HasTextBox() {
				for _, para := range shape.TextBox().Paragraphs() {
					for _, run := range para.Runs() {
						result.WriteString(run.Text())
					}
					result.WriteString("\n")
				}
			}
		}
		result.WriteString("\n")
	}
	return result.String(), nil
}

// ZipReader handles ZIP files
type ZipReader struct{}

func (r *ZipReader) Parse(filePath string) (string, error) {
	log.Printf("Reading ZIP file from %s", filePath)
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening ZIP file: %w", err)
	}
	defer reader.Close()

	var result strings.Builder
	fileReader := NewFileReader()
	
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		
		result.WriteString(fmt.Sprintf("File: %s\n", file.Name))
		
		rc, err := file.Open()
		if err != nil {
			result.WriteString(fmt.Sprintf("Error opening file: %v\n", err))
			continue
		}
		
		// Create temporary file to process
		tempFile, err := os.CreateTemp("", "zipextract_*")
		if err != nil {
			rc.Close()
			continue
		}
		
		_, err = io.Copy(tempFile, rc)
		rc.Close()
		tempFile.Close()
		
		if err == nil {
			content, err := fileReader.ReadFile(tempFile.Name())
			if err != nil {
				result.WriteString(fmt.Sprintf("Error reading file: %v\n", err))
			} else {
				result.WriteString(content)
			}
		}
		
		os.Remove(tempFile.Name())
		result.WriteString("\n---\n")
	}
	return result.String(), nil
}

// PythonReader handles Python files
type PythonReader struct{}

func (r *PythonReader) Parse(filePath string) (string, error) {
	log.Printf("Executing and reading Python file from %s", filePath)
	
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}
	
	var result strings.Builder
	result.WriteString("File Content:\n")
	result.WriteString(string(content))
	result.WriteString("\n\n")
	
	// Execute Python file
	cmd := exec.Command("python", filePath)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		result.WriteString("Execution Error:\n")
		result.WriteString(string(output))
	} else {
		result.WriteString("Execution Output:\n")
		result.WriteString(string(output))
	}
	
	return result.String(), nil
}

// FileReader is the main file reader that delegates to specific readers
type FileReader struct {
	readers map[string]Reader
}

// NewFileReader creates a new FileReader with all supported readers
func NewFileReader() *FileReader {
	return &FileReader{
		readers: map[string]Reader{
			".txt":      &TXTReader{},
			".csv":      &TXTReader{},
			".pdf":      &PDFReader{},
			".docx":     &DOCXReader{},
			".json":     &JSONReader{},
			".jsonld":   &JSONReader{},
			".jsonl":    &JSONLReader{},
			".xml":      &XMLReader{},
			".yaml":     &YAMLReader{},
			".yml":      &YAMLReader{},
			".html":     &HTMLReader{},
			".htm":      &HTMLReader{},
			".xhtml":    &HTMLReader{},
			".md":       &MarkdownReader{},
			".markdown": &MarkdownReader{},
			".xlsx":     &ExcelReader{},
			".xls":      &XLSReader{},
			".pptx":     &PPTXReader{},
			".zip":      &ZipReader{},
			".py":       &PythonReader{},
			".pdb":      &TXTReader{},
		},
	}
}

// ReadFile reads a file and returns its content as text
func (fr *FileReader) ReadFile(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	reader, exists := fr.readers[ext]
	if !exists {
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
	
	log.Printf("Reading file %s using %T", filePath, reader)
	return reader.Parse(filePath)
}

// GeneralReader provides a high-level interface similar to the Python version
type GeneralReader struct {
	fileReader *FileReader
	name       string
	description string
}

// NewGeneralReader creates a new GeneralReader
func NewGeneralReader() *GeneralReader {
	return &GeneralReader{
		fileReader: NewFileReader(),
		name:       "General File Reader",
		description: `A general file reader supporting formats: 'py', 'txt', 'csv', 'json', 
                      'jsonld', 'jsonl', 'yaml', 'yml', 'xlsx', 'xls', 'html', 'htm', 
                      'xml', 'pdf', 'docx', 'pptx', 'md', 'markdown', 'zip', 'pdb'.`,
	}
}

// Read reads a file and returns formatted content
func (gr *GeneralReader) Read(task, filePath string) (string, error) {
	content, err := gr.fileReader.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	ext = strings.TrimPrefix(ext, ".")
	
	var result strings.Builder
	result.WriteString(fmt.Sprintf("The %s file contains:\n---\n", ext))
	result.WriteString(content)
	result.WriteString("\n---")
	
	return result.String(), nil
}

// Example usage
func main() {
	reader := NewGeneralReader()
	
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run file_reader.go <file_path>")
		return
	}
	
	filePath := os.Args[1]
	content, err := reader.Read("describe the file", filePath)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	
	fmt.Println(content)
}