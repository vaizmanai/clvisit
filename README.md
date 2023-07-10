# Client Communicator for reVisit

## Notes

It contains two versions:

- **standalone** - can be run alone without additional UI part
- **communicator** - it is helper for UI implemented by C++

## Building

Building windows standalone and communicator versions:

```
make windows
```

Before start, you have to install fakeroot and g++
```
sudo apt install -y fakeroot g++ libgtk-3-dev libwebkit2gtk-4.0-dev
```

Building linux standalone, communicator versions and pack to deb:

```
make linux
```

***
server side
https://github.com/vaizmanai/srvisit

ui client side
https://github.com/vaizmanai/uivisit
