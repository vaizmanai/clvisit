# Client Communicator for reVisit

## Notes

It contains two versions:

- **standalone** - can be run alone without additional UI part
- **communicator** - it is helper for UI implemented by C++

## Building

Building windows standalone and communicator versions:

```
make
```

Building only windows standalone version:

```
make build_standalone
```

Building only windows communicator version:

```
make build_communicator
```

Building linux standalone version:

```
go build -o build/standalone -tags=webui clvisit/cmd/communicator
```

***
server side
https://github.com/vaizmanai/srvisit

ui client side
https://github.com/vaizmanai/uivisit
