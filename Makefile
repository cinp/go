all:

test-distros:
	echo xenial

test-requires:
	echo golint golang-1.10

test:
	go test -cover

lint:
	golint .

dpkg-distros:
	echo bionnic xenial

dpkg-requires:
	echo golang-1.10
