rm -r /sys/export/assets
cp -r /app/$1 /sys/export/assets
mkdir /sys/tmp/export
build -os $GOOS -arch $GOARCH -output /sys/tmp/export/$1 /sys/export
dl /sys/tmp/export/$1
