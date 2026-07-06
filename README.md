# Google Photos Metadata Fixer

A simple cli tool to restore metadata of media files from Google Photos downloaded through Google Takeout. It takes the edited variant (if available) and applies the creation date from metadata json file.

```sh
google-photos-takeout-fixer -i ./Takeout -o ./output
```

## Flags

- `-i`: Input directory
- `-o`: Output directory
- `-flat`: Dump all media into the output directory without preserving directory structure
