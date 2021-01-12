set curDir=%cd%
set GOPATH_BAK=%GOPATH%;
set GOPATH=%GOPATH%;%curDir%
go build -o bin/testChatRoom.exe chatroom


go fmt chatroom/...
set GOPATH_BAK=GOPATH
pause
