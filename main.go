package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/juju/fslock"
	"github.com/spf13/cobra"
)

type RecycleBin struct {
	trashPath string
	trashMap  map[string]string
}

func NewRecycleBin(trashPath string, trashMap map[string]string) *RecycleBin {
	return &RecycleBin{trashPath, trashMap}
}

func (rb *RecycleBin) Delete(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	fileName := fileInfo.Name()
	fileExt := filepath.Ext(fileName)
	fileBase := strings.TrimSuffix(fileName, fileExt)
	trashFileName := fmt.Sprintf("%s_%d%s", fileBase, time.Now().Unix(), fileExt)

	trashPath := filepath.Join(rb.trashPath, trashFileName)
	err = os.Rename(absPath, trashPath)
	if err != nil {
		return err
	}

	rb.trashMap[absPath] = trashPath

	fmt.Printf("Deleted %s, moved to %s\n", path, trashPath)
	return nil
}

func (rb *RecycleBin) Restore(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	trashFilePath, ok := rb.trashMap[absPath]
	if !ok {
		return nil
	}

	err = os.Rename(trashFilePath, absPath)
	if err != nil {
		return err
	}

	delete(rb.trashMap, absPath)

	fmt.Printf("Restored %s\n", absPath)
	return nil
}

func (rb *RecycleBin) GarbageCollect(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	err := filepath.Walk(rb.trashPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.ModTime().Unix() < cutoff {
			err = os.RemoveAll(path)
			if err != nil {
				return err
			}
			originalPath := rb.GetOriginalPath(path)
			if originalPath != "" {
				delete(rb.trashMap, originalPath)
				fmt.Printf("Removed %s\n", path)
			}
		}
		return nil
	})
	return err
}

func (rb *RecycleBin) List() error {
	for k := range rb.trashMap {
		fmt.Println(k)
	}
	return nil
}

func (rb *RecycleBin) SaveTrashMap(p string) error {
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	err = encoder.Encode(rb.trashMap)
	if err != nil {
		return err
	}
	return nil
}

func (rb *RecycleBin) GetOriginalPath(trashFile string) string {
	for k, v := range rb.trashMap {
		if v == trashFile {
			return k

		}
	}
	return ""
}

func loadTrashMap(trashMapPath string) (map[string]string, error) {
	file, err := os.Open(trashMapPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	trashMap := make(map[string]string)
	decoder := json.NewDecoder(file)
	_ = decoder.Decode(&trashMap)

	return trashMap, nil
}

func initTrashMap(trashMapPath string) error {
	_, err := os.Stat(trashMapPath)
	if os.IsNotExist(err) {
		// Create the directory for the trash map file if it doesn't exist
		err = os.MkdirAll(filepath.Dir(trashMapPath), 0755)
		if err != nil {
			return err
		}

		// Create an empty trash map file
		file, err := os.Create(trashMapPath)
		if err != nil {
			return err
		}
		defer file.Close()
		return nil
	}

	return err
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Failed to get user home directory:", err)
		os.Exit(1)
	}
	trashPath := filepath.Join(homeDir, ".trash")
	trashMapPath := filepath.Join(trashPath, ".trashmap.json")
	err = initTrashMap(trashMapPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	lock := fslock.New(trashMapPath)
	lockErr := lock.TryLock()
	if lockErr != nil {
		fmt.Println("falied to acquire lock > " + lockErr.Error())
		return
	}
	defer lock.Unlock()
	trashMap, err := loadTrashMap(trashMapPath)
	if err != nil {
		fmt.Println("failed to load trashMap:", err)
		os.Exit(1)
	}
	rb := NewRecycleBin(trashPath, trashMap)

	var rootCmd = &cobra.Command{
		Use:     "rmbin",
		Version: "v0.0.2",
	}

	var deleteCmd = &cobra.Command{
		Use:     "delete [files...]",
		Aliases: []string{"d"},
		Short:   "Move files to recycle bin",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				err := rb.Delete(arg)
				if err != nil {
					fmt.Println(err)
				}
			}
			return rb.SaveTrashMap(trashMapPath)
		},
	}

	var restoreCmd = &cobra.Command{
		Use:     "restore [files...]",
		Aliases: []string{"r"},
		Short:   "Restore files from recycle bin",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				err := rb.Restore(arg)
				if err != nil {
					fmt.Println(err)
				}
			}
			return rb.SaveTrashMap(trashMapPath)
		},
	}

	var garbageCollectCmd = &cobra.Command{
		Use:   "gc [days]",
		Short: "Clean up the recycle bin",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			days := 30
			if len(args) == 1 {
				var err error
				days, err = strconv.Atoi(args[0])
				if err != nil {
					return err
				}
			}

			err := rb.GarbageCollect(days)
			if err != nil {
				fmt.Println(err)
			}
			return rb.SaveTrashMap(trashMapPath)
		},
	}

	var listCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists the recycle bin files",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := rb.List()
			if err != nil {
				fmt.Println(err)
			}
			return nil
		},
	}

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(garbageCollectCmd)

	err = rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
