rm -r /sys/export/assets
cp -r $2 /sys/export/assets
mkdir /sys/tmp/export
build -os $GOOS -arch $GOARCH -output /sys/tmp/export/$1 /sys/export
dl /sys/tmp/export/$1
