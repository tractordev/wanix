cp -r /app/$1 /sys/export
mv /sys/export/$1 /sys/export/assets 
GOOS=darwin GOARCH=amd64 build /sys/export/main.go
mv /sys/export/export /sys/export/$1
dl /sys/export/$1
