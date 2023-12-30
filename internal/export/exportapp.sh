rm -r /sys/export/assets 
cp -r /app/$1 /sys/export/assets 
build -os $GOOS -arch $GOARCH -output /sys/export/$1 /sys/export/main.go
dl /sys/export/$1
