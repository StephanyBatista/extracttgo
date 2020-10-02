package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	guuid "github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

type fileToProcess struct {
	URL string `json:"url" binding:"required"`
}

func main() {

	r := gin.Default()
	r.POST("/process", func(c *gin.Context) {

		var json fileToProcess
		if err := c.Bind(&json); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, err := processFile(json.URL)
		if err != nil {
			fmt.Println("process", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)

	})
	r.Run(":3001")
}

func processFile(url string) (map[int]string, error) {

	guid := guuid.New().String()
	filePath := guid[0:8] + ".pdf"
	if fileErr := downloadFile(filePath, url); fileErr != nil {
		fmt.Println("processFile 1", fileErr.Error(), filePath)
		return nil, fileErr
	}
	f, r, err := pdf.Open(filePath)

	if err != nil {
		fmt.Println("processFile 2", err.Error(), filePath)
		return nil, err
	}
	defer f.Close()
	defer deleteFile(filePath)

	textPerPage := make(map[int]string, r.NumPage())
	var wg sync.WaitGroup

	for i := 1; i <= r.NumPage(); i++ {
		wg.Add(1)
		go func(page int, wg *sync.WaitGroup) {
			text, _ := processPage(filePath, page)
			textPerPage[page] = text
			defer wg.Done()
		}(i, &wg)
	}

	wg.Wait()

	return textPerPage, nil
}

func processPage(filePath string, page int) (string, error) {

	filePagePath := strconv.Itoa(page) + filePath
	createPage(filePath, filePagePath, page)
	text, err := readPage(filePagePath)
	defer deleteFile(filePagePath)
	if err != nil {
		panic(err)
		fmt.Println("processPage: ", err.Error(), filePagePath)
		return "", err
	}
	return text, nil
}

func createPage(filePath string, filePagePath string, page int) error {
	currentPath, _ := os.Getwd()
	pdfPath := currentPath + "/" + filePath
	pagePath := currentPath + "/" + filePagePath

	cmd := exec.Command("sh", "-c", "qpdf "+pdfPath+" --pages . "+strconv.Itoa(page)+" -- "+pagePath)
	if err := cmd.Run(); err != nil {
		fmt.Println("createPage: ", err.Error(), pdfPath)
		return err
	}
	return nil
}

func readPage(filePath string) (string, error) {
	currentPath, _ := os.Getwd()
	pdfPath := currentPath + "/" + filePath

	out, err := exec.Command("sh", "-c", "pdftotext "+pdfPath+" -").Output()
	if err != nil {
		fmt.Println("ReadPage: ", err.Error(), pdfPath)
		return "", err
	}
	return string(out), nil
}

func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Println("downloadFile status code: ", resp.StatusCode)
	}
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func deleteFile(filepath string) error {

	if err := os.Remove(filepath); err != nil {
		return err
	}
	return nil
}
