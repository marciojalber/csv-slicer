@echo off

:: BUILDING BUILD
    set bin=csvslicer.exe 

:: COMPILE
    echo.
    echo COMPILING...
    go build -ldflags="-w -s" -o %bin% code/main.go

:: EXECUTE
    echo.
    echo EXECUTING...
    %bin%
