package filereaders

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

	"github.com/PuerkitoBio/goquery"
	"github.com/ledongthuc/pdf"
	"github.com/russross/blackfriday/v2"
	"github.com/tealeg/xlsx"
	"github.com/unidoc/unioffice/document"
	"github.com/unidoc/unioffice/presentation"
	"github.com/xuri/excelize/v2"
	"gopkg.in/yaml.v2"
)

// Reader interface defines the contract for all file readers
type Reader interface {
	Parse(filePath string) (string, error)
}

// TXTReader handles plain text files
type TXTReader struct{}
func (r *TXTReader) Parse(filePath string) (string, error) {
	//log.Printf("Reading TXT file from %s", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}
	
	// Limit content to 20000 characters if it's longer
	if len(content) > 20000 {
		return string(content[:20000]), nil
	}
	
	return string(content), nil
}

// PDFReader handles PDF files
type PDFReader struct{}

func (r *PDFReader) Parse(filePath string) (string, error) {
	log.Printf("Reading PDF file from %s", filePath)
	// pdf.Open 是一个辅助函数，它会打开文件并创建读取器
	// 重要的是要关闭它返回的文件句柄
	f, rdr, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening PDF: %w", err)
	}
	// 使用 defer 来确保文件在函数结束时被关闭
	defer f.Close()

	var sb strings.Builder // 使用 strings.Builder 来高效拼接字符串
	numPages := rdr.NumPage()

	// 遍历 PDF 的所有页面
	for i := 1; i <= numPages; i++ {
		page := rdr.Page(i)
		if page.V.IsNull() {
			continue // 跳过无效或空的页面
		}

		// 这是更可靠的文本提取方法
		// 它会自动解析内容流并返回一个包含文本块的切片
		texts := page.Content().Text
		for _, t := range texts {
			sb.WriteString(t.S)
		}
		// 在每页内容后添加一个换行符，以保持基本的段落分隔
		sb.WriteString("\n")
	}

	// 检查是否提取到了内容
	if sb.Len() == 0 {
		log.Printf("Warning: No text was extracted from %s. The document might be image-based or have an unusual structure.", filePath)
	}

	return sb.String(), nil
}

type DOCXReader struct{}

func (r *DOCXReader) Parse(filePath string) (string, error) {
	txtFilePath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".txt"
	if _, err := os.Stat(txtFilePath); err == nil {
		txtReader := &TXTReader{}
		return txtReader.Parse(txtFilePath)
	}

	log.Printf("Reading DOCX file from %s", filePath)

	// 使用 unidoc/unioffice 库打开文档
	doc, err := document.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening DOCX file: %w", err)
	}

	var sb strings.Builder // 使用 strings.Builder 来高效拼接字符串

	// 遍历文档中的所有段落
	for _, p := range doc.Paragraphs() {
		// 遍历段落中的所有文本块 (run)
		for _, r := range p.Runs() {
			sb.WriteString(r.Text()) // 提取并拼接文本
		}
		sb.WriteString("\n") // 每个段落后添加换行符
	}

	return sb.String(), nil
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

	// Parse XML and extract only text content
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var result strings.Builder

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// If XML parsing fails, return raw content
			log.Printf("XML parsing failed, returning raw content: %v", err)
			return string(content), nil
		}

		switch t := token.(type) {
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				result.WriteString(text)
				result.WriteString(" ")
			}
		}
	}

	// Clean up extra spaces and return
	finalText := strings.TrimSpace(result.String())

	// If we successfully parsed but got empty result, return raw content
	if finalText == "" {
		return string(content), nil
	}

	return finalText, nil
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
		// If xlsx library fails, try to read as binary/text content
		log.Printf("XLS file may be in older binary format, attempting basic read: %v", err)
		content, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return "", fmt.Errorf("error reading XLS file: %w", readErr)
		}

		// Try to extract meaningful text from binary content
		var result strings.Builder
		var words []string

		// Simple text extraction - look for sequences of printable characters
		var currentWord strings.Builder
		for _, b := range content {
			if b >= 32 && b <= 126 { // printable ASCII
				currentWord.WriteByte(b)
			} else {
				if currentWord.Len() >= 3 { // only keep words of 3+ chars
					word := currentWord.String()
					// Filter out common Excel metadata and formatting junk
					if !isExcelJunk(word) && isValidWord(word) {
						words = append(words, word)
					}
				}
				currentWord.Reset()
			}
		}
		if currentWord.Len() >= 3 {
			word := currentWord.String()
			if !isExcelJunk(word) && isValidWord(word) {
				words = append(words, word)
			}
		}

		// Remove duplicates and join meaningful words
		uniqueWords := removeDuplicates(words)
		if len(uniqueWords) > 0 {
			result.WriteString(strings.Join(uniqueWords, " "))
		} else {
			result.WriteString("No readable text content found in this XLS file.")
		}

		return result.String(), nil
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

// Helper function to check if a word is Excel formatting junk
func isExcelJunk(word string) bool {
	junkPatterns := []string{
		"###", "000", "???", "___", "---", "###",
		"Calibri", "Arial", "Times", "Verdana", "Tahoma",
		"RGB", "XML", "PK", "ZIP", "META", "DOCX", "XLSX",
		"Content_Types", "_rels", "workbook", "worksheet",
		"theme", "styles", "sharedStrings", "drawing",
		"Microsoft", "Office", "Word", "Excel", "PowerPoint",
		"xmlns", "http", "www", "schemas", "com",
	}

	for _, pattern := range junkPatterns {
		if strings.Contains(strings.ToUpper(word), strings.ToUpper(pattern)) {
			return true
		}
	}

	// Filter out strings that are mostly symbols or numbers
	alphaCount := 0
	for _, r := range word {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			alphaCount++
		}
	}

	// Word should be at least 50% alphabetic characters
	return float64(alphaCount)/float64(len(word)) < 0.5
}

// Helper function to check if a word contains valid readable content
func isValidWord(word string) bool {
	// Must be at least 3 characters
	if len(word) < 3 {
		return false
	}

	// Must contain at least one letter
	hasLetter := false
	for _, r := range word {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
			break
		}
	}

	return hasLetter
}

// Helper function to remove duplicate words while preserving order
func removeDuplicates(words []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, word := range words {
		if !seen[word] {
			seen[word] = true
			result = append(result, word)
		}
	}

	return result
}

// PPTXReader handles PowerPoint files
type PPTXReader struct{}

func (r *PPTXReader) Parse(filePath string) (string, error) {
	txtFilePath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".txt"
	if _, err := os.Stat(txtFilePath); err == nil {
		txtReader := &TXTReader{}
		return txtReader.Parse(txtFilePath)
	}

	log.Printf("Reading PowerPoint file from %s", filePath)
	pres, err := presentation.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening PowerPoint file: %w", err)
	}
	defer pres.Close()

	var result strings.Builder

	// 使用 ExtractText 方法获取文本提取器
	pe := pres.ExtractText()

	// 获取所有文本作为纯文本
	result.WriteString("=== Full Text Content ===\n")
	result.WriteString(pe.Text())
	result.WriteString("\n\n")

	// 按幻灯片分别处理
	result.WriteString("=== Slide by Slide Content ===\n")
	for slideIndex, slide := range pe.Slides {
		result.WriteString(fmt.Sprintf("Slide %d:\n", slideIndex+1))

		for itemIndex, item := range slide.Items {
			result.WriteString(fmt.Sprintf("  Item %d: %s\n", itemIndex+1, item.Text))

			// 可选：添加格式信息
			if item.Run != nil && item.Run.RPr != nil {
				runProps := item.Run.RPr
				var formatInfo []string

				if runProps.BAttr != nil {
					formatInfo = append(formatInfo, "Bold")
				}
				if runProps.IAttr != nil {
					formatInfo = append(formatInfo, "Italic")
				}
				if runProps.SzAttr != nil {
					formatInfo = append(formatInfo, fmt.Sprintf("Size: %d", *runProps.SzAttr/100))
				}

				if len(formatInfo) > 0 {
					result.WriteString(fmt.Sprintf("    Format: %s\n", strings.Join(formatInfo, ", ")))
				}
			}

			// 处理表格信息
			if tblInfo := item.TableInfo; tblInfo != nil {
				result.WriteString(fmt.Sprintf("    Table: Row %d, Column %d\n", tblInfo.RowIndex, tblInfo.ColIndex))
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

		// Create temporary file with original extension to preserve file type
		ext := filepath.Ext(file.Name)
		tempFile, err := os.CreateTemp("", "zipextract_*"+ext)
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
	fileReader  *FileReader
	name        string
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

// Example usage - commented out since this is now a library package
/*
func main() {
	reader := NewGeneralReader()
	filePath := "../../dataset/gaia/val_files/a3fbeb63-0e8c-4a11-bff6-0e3b484c3e9c.pptx"
	// filePath := "../../dataset/gaia/val_files/9b54f9d9-35ee-4a14-b62f-d130ea00317f.zip"
	content, err := reader.Read("describe the file", filePath)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	fmt.Println(content)
}
*/
