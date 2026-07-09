# Google Photos Metadata Fixer

> [!CAUTION]
> This project has been vibe-coded.

A simple cli tool to restore metadata of media files from Google Photos downloaded through Google Takeout. It takes the edited variant (if available) and applies the creation date from metadata json file.

```sh
google-photos-takeout-fixer -i ./Takeout -o ./output
```

## Flags

- `-i`: Input directory
- `-o`: Output directory
- `-flat`: Dump all media into the output directory without preserving directory structure

## Internals

1. Find metadata json files
2. Read title and find corresponding picture
3. `-edited` variant? -> take that one
4. Starts with `motion_`? -> take video over image

_Note: Due to Google Takeout's compression, some actual file names have been truncated to 50/51 characters_
