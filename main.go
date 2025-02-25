package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-darwin/apfs"
)

var hashLookup = map[string][]string{}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: main <directory>")
		os.Exit(1)
	}

	// Walk input directory
	err := filepath.WalkDir(os.Args[1], func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if strings.Contains(err.Error(), "operation not permitted") {
				return nil
			}

			return fmt.Errorf("error opening dir %w", err)
		}

		if d.IsDir() {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			err = fmt.Errorf("error hashing file %s: %w", path, err)
			log.Println(err)
			return nil
		}

		hashLookup[hash] = append(hashLookup[hash], path)
		return nil
	})
	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	var totalSavings int64

	// Print duplicates
	for hash, paths := range hashLookup {
		if len(paths) <= 1 {
			continue
		}

		stats, err := os.Stat(paths[0])
		if err != nil {
			fmt.Printf("Error getting stats for %s: %v\n", paths[0], err)
			continue
		}

		fmt.Printf("Duplicate files with hash %s:\n", hash)

		// Calculate estimated savings if deduplicated
		estimatedSavings := stats.Size() * int64(len(paths)-1)
		totalSavings += estimatedSavings
		fmt.Printf("Estimated savings: %s\n", prettyPrintBytes(estimatedSavings))

		for _, path := range paths {
			fmt.Println(path)
		}
	}

	fmt.Println("Total Estimated Savings:", prettyPrintBytes(totalSavings))

	// Ask if they'd like to continue
	fmt.Print("Do you want to continue? (y/n): ")
	var choice string
	fmt.Scanln(&choice)
	if choice != "y" {
		fmt.Println("Exiting...")
		os.Exit(0)
	}

	// Continue with migration
	fmt.Println("Starting migration...")
	err = migrate(hashLookup)
	if err != nil {
		fmt.Printf("Error migrating duplicates: %v\n", err)
		os.Exit(1)
	}
}

func migrate(hashLookup map[string][]string) error {
	for _, paths := range hashLookup {
		if len(paths) <= 1 {
			continue
		}

		processDuplicates(paths)

	}
	return nil
}

func processDuplicates(paths []string) error {
	for i, path := range paths {
		// Keep first file.
		if i == 0 {
			continue
		}

		// Stat file to get permission info
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("error getting file info for %s: %v", path, err)
		}

		// Move file to backup before attempting clone so we can restore if something fails.
		err = os.Rename(path, fmt.Sprintf("%s.bak", path))
		if err != nil {
			return fmt.Errorf("error moving file %s to backup directory: %v", path, err)
		}

		err = apfs.CloneFile(paths[0], path, apfs.CLONE_NOFOLLOW)
		switch {
		case err == nil:
		default:
			log.Println("error cloning file", err)
			// rollback the backup file
			if err := os.Rename(fmt.Sprintf("%s.bak", path), path); err != nil {
				log.Println("error restoring backup file", err)
			}
			continue
		}

		// Restore permissions to original file
		if err := os.Chmod(path, info.Mode()); err != nil {
			log.Println("error restoring permissions for file", path, err)
		}

		// Restore ownership to original file
		if err := os.Chown(path, int(info.Sys().(*syscall.Stat_t).Uid), int(info.Sys().(*syscall.Stat_t).Gid)); err != nil {
			log.Println("error restoring ownership for file", path, err)
		}

		// err = os.Remove(path)
		// if err != nil {
		// 	return fmt.Errorf("error removing file %s: %v", path, err)
		// }
	}

	return nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func prettyPrintBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
}
