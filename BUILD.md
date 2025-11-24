# Building sword-tui

## Build Script

Use the provided build script to automatically increment the build number with each build:

```bash
./build.sh
```

This will:
1. Increment the build number stored in `.build_number`
2. Compile the application with the build number injected
3. Output the build number in the footer of the application

## Manual Build

If you want to build without incrementing the build number:

```bash
go build -o sword-tui ./cmd/sword-tui
```

This will show "dev" as the build number.

## Build with Custom Build Number

You can also manually specify a build number:

```bash
go build -ldflags "-X sword-tui/internal/version.BuildNumber=123" -o sword-tui ./cmd/sword-tui
```

## Version Display

The version is displayed in the footer of the application in the format:
```
v1.0.0 (build <number>)
```

Where:
- `v1.0.0` is the version defined in `internal/version/version.go`
- `<number>` is the build number (either auto-incremented, manually specified, or "dev")
