package vies

import (
	"archive/zip"
	"bytes"
	"cmp"
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
)

const (
	typeSharedStrings = "application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"
	typeWorkSheet     = "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"
)

type xmlSharedStrings struct {
	XMLName     xml.Name `xml:"sst"`
	Count       int      `xml:"count,attr"`
	UniqueCount int      `xml:"uniqueCount,attr"`
	Strings     []string `xml:"si>t"`
}

type xmlContentTypes struct {
	Types     xml.Name `xml:"Types"`
	Overrides []struct {
		PartName    string `xml:"PartName,attr"`
		ContentType string `xml:"ContentType,attr"`
	} `xml:"Override"`
}

type xmlWorkSheet struct {
	XMLName xml.Name  `xml:"worksheet"`
	Rows    []*xmlRow `xml:"sheetData>row"`
}

type xmlRow struct {
	Index int        `xml:"r,attr"`
	Cells []*xmlCell `xml:"c"`
}
type xmlCell struct {
	Type     string `xml:"t,attr"`
	RowIndex string `xml:"r,attr"`
	Index    int
	Value    string `xml:"v"`
	String   string `xml:"is>t"`
}

type SpreadsheetMlReader struct {
}

func (s *SpreadsheetMlReader) Handle(content *[]byte) ([][]string, error) {

	archive, err := zip.NewReader(bytes.NewReader(*content), int64(len(*content)))
	if err != nil {
		return nil, err
	}

	var firstSheetName string
	var sharedStringsName string

	for _, f := range archive.File {
		if f.Name == "[Content_Types].xml" {

			var contentTypes xmlContentTypes
			if err := s.readXmlToStruct(f, &contentTypes); err != nil {
				return nil, err
			}

			for _, f := range contentTypes.Overrides {
				switch f.ContentType {
				case typeSharedStrings:
					sharedStringsName = strings.TrimPrefix(f.PartName, "/")
				case typeWorkSheet:
					firstSheetName = strings.TrimPrefix(f.PartName, "/")
				}
			}

		}
	}

	if firstSheetName == "" {
		return nil, errors.New("no sheets found")
	}

	var worksheet xmlWorkSheet
	var sharedStrings xmlSharedStrings

	for _, f := range archive.File {
		if f.Name == sharedStringsName {
			if err := s.readXmlToStruct(f, &sharedStrings); err != nil {
				return nil, err
			}
			continue
		}
		if f.Name == firstSheetName {
			if err := s.readXmlToStruct(f, &worksheet); err != nil {
				return nil, err
			}
			slices.SortFunc(worksheet.Rows, func(a, b *xmlRow) int {
				return cmp.Compare(a.Index, b.Index)
			})
			continue
		}
	}

	maxColumnIndex := -1
	for _, row := range worksheet.Rows {
		for _, cell := range row.Cells {
			cell.Index = s.decodeOneBasedBase26(strings.TrimRight(cell.RowIndex, "1234567890")) - 1
			if cell.Index > maxColumnIndex {
				maxColumnIndex = cell.Index
			}
		}
		slices.SortFunc(row.Cells, func(a, b *xmlCell) int {
			return cmp.Compare(a.Index, b.Index)
		})
	}

	if maxColumnIndex < 0 {
		return nil, errors.New("no columns found")
	}

	result := make([][]string, len(worksheet.Rows))

	for rowIdx, row := range worksheet.Rows {
		cellData := make([]string, maxColumnIndex+1)
		for _, cell := range row.Cells {

			if cell.Index < 0 || cell.Index >= maxColumnIndex+1 {
				continue
			}

			cellData[cell.Index] = cell.String
			switch cell.Type {
			case "s":
				idx, err := strconv.Atoi(cell.Value)
				if err != nil || idx < 0 || idx >= len(sharedStrings.Strings) {
					continue
				}
				cellData[cell.Index] = sharedStrings.Strings[idx]
			case "str":
				cellData[cell.Index] = cell.Value
			}
		}
		result[rowIdx] = cellData
	}

	return result, nil
}

func (s *SpreadsheetMlReader) readXmlToStruct(f *zip.File, dst interface{}) error {

	h, err := f.Open()
	if err != nil {
		return err
	}
	defer h.Close()

	content, err := io.ReadAll(h)
	if err != nil {
		return err
	}

	err = xml.Unmarshal(content, dst)
	if err != nil {
		return err
	}

	return nil
}

func (s *SpreadsheetMlReader) decodeOneBasedBase26(str string) int {
	str = strings.ToUpper(str)
	if str == "" {
		return -1
	}
	result := 0
	for _, ch := range str {
		if ch < 'A' || ch > 'Z' {
			return -1
		}
		value := int(ch-'A') + 1
		result = result*26 + value
	}
	return result
}
