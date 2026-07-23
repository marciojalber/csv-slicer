@echo off

:: BUILD
    set bin=csvslicer.exe

:: COMPILE PAGES
    echo.
    echo COMPILING PAGES...
    go run cmd/jf_ui/ui.go

:: COMPILE APP
    echo.
    echo COMPILING APP...
    go mod tidy
    : go build -ldflags="-w -s" -o %bin% main.go
    go build -ldflags="-w -s" -o %bin% main.go

:: EXECUTE
    echo.
    echo EXECUTING...
    %bin%
