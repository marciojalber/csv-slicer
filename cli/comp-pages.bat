@echo off

:: COMPILE PAGES
    echo.
    echo COMPILING PAGES...
    go run cmd/jf_ui/ui.go
    
    :: rsrc -ico pngui/img/favicon.ico
