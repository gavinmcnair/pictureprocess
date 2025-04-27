# Media Deduplication and Organization Tool

This tool processes and organizes media files from a given source directory into a destination directory. It handles images, RAW files, and video files, deduplicating them based on content or file size and ensuring that binary integrity is maintained.

## Features

- **Image Processing**: Supports standard image formats (`.jpg`, `.jpeg`, `.png`) with deduplication based on perceptual hashing.
- **RAW File Support**: Includes support for multiple RAW formats (`.nef`, `.cr2`, `.arw`, etc.), deduplicating based on file size similar to video files.
- **Video File Processing**: Supports common video formats (`.avi`, `.mp4`, `.mkv`, `.mov`) with deduplication via file size.
- **Directory-based Organization**: Files are organized into directories named by their extracted ISO date.
- **Mapping in `index.json`**: Each directory contains an `index.json` mapping original file paths to their new names in the destination.

## Usage

```shell
./dedup <source_directory> <destination_directory>
```

- `<source_directory>`: The root directory containing media files to be organized and processed.
- `<destination_directory>`: The target location where processed files and corresponding `index.json` mappings will be stored.

## Installation

1. Clone the repository:
   ```shell
   git clone git@github.com:gavinmcnair/pictureprocess.git
   cd pictureprocess
   ```

2. Build the application:
   ```shell
   go build -o dedup cmd/main.go
   ```

3. Run the application with the source and destination directories:
   ```shell
   ./dedup /path/to/source /path/to/destination
   ```

## Handling of File Types

- **Images**: Perceptual hashes are computed to check for duplicates. Files are organized by their capture date, extracted from metadata if available, or file properties.
  
- **RAW Files**: Comparison is handled by file size due to processing limitations, organized similarly to images.

- **Videos**: Also deduplicated on file size. Video metadata is utilized when possible.

## Output

- **Processed Summary**: After execution, the tool outputs the total count of processed, copied, and duplicated files for each media type.
- **`index.json`**: In each target directory, an `index.json` file is created, mapping each original file's relative path to its new filename. This assists in potential future operations like renaming or reverse mapping.

## Dependencies

- This tool is written in Go and depends on the following packages:
  - `github.com/corona10/goimagehash` for perceptual hashing
  - `github.com/disintegration/imaging` for image processing

## Notes

- Ensure the tool has write permissions in the destination directory.
- It's recommended to backup your media files prior to running for the first time.
- The tool efficiently leverages multiple CPU cores to process files concurrently, specified by the available number of cores on the machine.
