# go-dedupe

Go dedupe is a tool for identifying and merging duplicate files. This works by relying on APFS (Apple File System) snapshots and hard links.
This allows us to hash the contents of files to identify duplicates and then have cloned files point to the original file, maintaining a single copy of the file.

### Usage
```bash
  go-dedupe <directory>
```
This will scan the specified directory for duplicate files and give a preview of the files that will be merged and the estimated space savings.
