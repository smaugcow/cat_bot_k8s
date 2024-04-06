package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	// Token variable for authorization
	token string = "TOKEN"
	// The production chat ID
	chat_id_prod int64 = -777
	// The test chat ID
	chat_id_test int64 = -666
	// Local directory to store GIFs
	local_storage_dir string = "local_storage"

	// File name to store the information of the last sent file
	last_send_file_name string = "last_saved"

	// Default duration for time intervals
	duration time.Duration = 20 * time.Second

	// Flag to enable or disable debug mode
	debug bool = false
)

// saveLastSavedFile writes the file name of the last sent file
func saveLastSavedFile(filename string) error {
	return os.WriteFile(last_send_file_name, []byte(filename), 0666)
}

// getLastSavedFile reads the file name of the last sent file
func getLastSavedFile() (string, error) {
	data, err := os.ReadFile(last_send_file_name)
	return string(data), err
}

// getEarliestFile returns the oldest file (based on modification time) in the provided folder
func getEarliestFile(folder string) (os.FileInfo, error) {
	log.Printf("getEarliestFile: %v \n", folder)
	// Read all files in the provided folder
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	log.Printf("getEarliestFile: len(files): %v \n", len(files))
	// Sort the files based on their modification time
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})
	log.Printf("EarliestFile: %v \n", files[0])
	// Return the oldest file and no error
	return files[0], nil
}

// lastSavedFileInfo returns FileInfo for the last saved file.
// If the file does not exist, it finds the oldest file in the local storage directory.
func lastSavedFileInfo(lastSavedFile string) (os.FileInfo, error) {
	// Get FileInfo for the last saved file
	lastSavedFileInfo, err := os.Stat(local_storage_dir + "/" + lastSavedFile)
	if err != nil {
		// Check if the file does not exist
		if os.IsNotExist(err) {
			log.Printf("lastSavedFile NotExist: %v\n", lastSavedFile)
			// Get the oldest file in the local storage directory if last saved file does not exist
			earliestFile, err := getEarliestFile(local_storage_dir)
			if err != nil {
				return nil, err
			}
			// If no files are found in the directory, log the message and return nil
			if earliestFile == nil {
				log.Print("No files found")
				return nil, nil
			}
			log.Printf("earliestFile.Name(): %v\n", earliestFile.Name())
			return os.Stat(local_storage_dir + "/" + earliestFile.Name())
		}
		return nil, err
	}
	// Return FileInfo for the last saved file
	return lastSavedFileInfo, nil
}

func main() {
	// Create a new telegram bot API with the token
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		// If there's an error in bot creation, panic
		log.Panic(err)
	}

	// Set bot in debug mode as per the debug constant value
	bot.Debug = debug
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Initialize update configuration
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Get updates channel from the bot
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		// Log fatal error if any
		log.Fatal(err)
	}

	// Check if the last file sent exists, if not create it
	if _, err := os.Stat(last_send_file_name); os.IsNotExist(err) {
		os.WriteFile(last_send_file_name, []byte(""), 0666)
	}

	// Check if the local storage directory exists, if not create it
	if _, err := os.Stat(local_storage_dir); os.IsNotExist(err) {
		err := os.Mkdir(local_storage_dir, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Tick every duration
	ticker := time.NewTicker(duration)
	quit := make(chan struct{})

	// Start a goroutine for handling updates
	go func() {
		for {
			select {
			case <-ticker.C: // On every tick
				// Get the last saved file
				lastSavedFile, err := getLastSavedFile()
				if err != nil {
					log.Fatal(err)
				}

				log.Printf("lastSavedFile: %v\n", lastSavedFile)

				// Get the information for the last saved file
				lastSavedFileInfo, err := lastSavedFileInfo(lastSavedFile)
				if err != nil {
					log.Fatal(err)
				}
				// If there are no files, move to next ticker
				if lastSavedFileInfo == nil {
					continue
				}

				log.Printf("lastSavedFileInfo: %v\n", lastSavedFileInfo.Name())

				// Read the directory to get all the files
				files, _ := ioutil.ReadDir(local_storage_dir)
				log.Printf("go func(): len(files): %v\n", len(files))
				if len(files) == 0 {
					continue
				}

				// Sort the files by modification time
				sort.Slice(files, func(i, j int) bool {
					return files[i].ModTime().Before(files[j].ModTime())
				})

				// Get the first file
				var nextFile os.FileInfo = files[len(files)-1]
				// Iterate over the files and find the suitable file to be the next one
				for _, file := range files {
					log.Printf("for _, file := range files: %v\n", file.Name())
					if file.Name() != lastSavedFile && (lastSavedFile == "" || file.ModTime().After(lastSavedFileInfo.ModTime())) {
						nextFile = file
						break
					}
				}

				log.Printf("nextFile: %v\n", nextFile)

				// If nextFile is found
				if nextFile != nil {
					// Upload it to the test chat
					msg := tgbotapi.NewDocumentUpload(chat_id_test, local_storage_dir+"/"+nextFile.Name())
					_, err = bot.Send(msg)
					if err != nil {
						log.Println(err)
					}
					// Save this as the last sent file
					saveLastSavedFile(nextFile.Name())
				}

			case <-quit: // If the quit channel is signalled
				ticker.Stop() // Stop the ticker
				return        // Return from the goroutine, effectively killing it
			}
		}
	}()

	// For each update received
	for update := range updates {
		// If the update contains a message
		if update.Message != nil {
			log.Printf("update.Message: %v\n", update.Message)
			// If the message contains a document
			if update.Message.Document != nil {
				log.Printf("update.Message.Document: %v\n", update.Message.Document)
				log.Printf("update.Message.Document.MimeType: %v\n", update.Message.Document.MimeType)
				// If the MIME type of the document is of video/mp4 type
				if update.Message.Document.MimeType == "video/mp4" {
					// Get the FileID of the document
					fileID := update.Message.Document.FileID
					log.Printf("Get video/mp4 as image/gif file. fileID: %v\n", fileID)
					// Attempt to get the file with the file id from the bot
					file, err := bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
					if err != nil {
						// Log error and continue to the next update if an error occurred
						log.Printf("Failed to get FileID: %v\n", err)
						continue
					}

					// Construct the url to download the file
					url := "https://api.telegram.org/file/bot" + bot.Token + "/" + file.FilePath
					// Attempt to get the file from the url
					response, err := http.Get(url)
					if err != nil {
						// Log fatal error and continue to the next update if an error occurred
						log.Fatal(err)
						continue
					}
					// Ensure the response body is closed once done
					defer response.Body.Close()

					// Create the final path for the saved file
					final_path := local_storage_dir + "/" + fileID + ".mp4"
					log.Printf("try save GIF: %v\n", final_path)
					// Attempt to create the file at the final path
					output, err := os.Create(final_path)
					if err != nil {
						// Log fatal error and continue to the next update if an error occurred
						log.Fatal(err)
						continue
					}
					// Ensure the newly created file is closed once done
					defer output.Close()

					// Copy the file data from the response body to the file
					_, err = io.Copy(output, response.Body)
					if err != nil {
						// Log fatal error and continue to the next update if an error occurred
						log.Fatal(err)
						continue
					}
					// Send a new message to the chat where the document came from
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "good giiif")
					bot.Send(msg)
				} else {
					// If the document MIME type is not an "video/mp4", send a message to the chat
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "if not froging gif")
					bot.Send(msg)
				}
			}
		}
	}
}
