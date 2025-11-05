# protocGenWebviewRpc

## Pre-requirements
### Install protoc
**Mac**
```shell
brew install protobuf
protoc --version  # Ensure compiler version is 3+
```
**Windows**
```shell
winget install protobuf
protoc --version # Ensure compiler version is 3+
```

**Linux**
```shell
apt install -y protobuf-compiler
protoc --version  # Ensure compiler version is 3+
```

## Usage
### Basic Usage

**Format**
```shell
protoc \
  --plugin=protoc-gen-webviewrpc=<path to protoc-gen-webviewrpc> \
  --webviewrpc_out=<generating options>:<output directory> \
  -I. <proto file>
```

**Javascript**
```shell
npx pbjs HelloWorld.proto --es6 hello_world.js
```

**CSharp**
```shell
protoc --csharp_out=. HelloWorld.proto
```


### Generate CSharp Server Code
```shell
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=cs_server:./OutCSharp \
  -I. my_service.proto
```

### Generate CSharp Client Code
```shell
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=cs_client:./OutCSharp \
  -I. my_service.proto
```

### Generate JavaScript Server Code
```shell
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=js_server:./OutJavaScript \
  -I. my_service.proto
```

### Generate JavaScript Client Code
```shell
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=js_client:./OutJavaScript \
  -I. my_service.proto
```

### Generate TypeScript Server Code (v2.1.0+)
```shell
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=ts_server:./OutTypeScript \
  -I. my_service.proto
```

### Generate TypeScript Client Code (v2.1.0+)
```shell
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=ts_client:./OutTypeScript \
  -I. my_service.proto
```

### Generate Multiple Code
```shell
# All languages (v2.1.0+)
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=cs_client,cs_server,js_client,js_server,ts_client,ts_server:./All \
  -I. my_service.proto

# C# and JavaScript only
protoc \
  --plugin=protoc-gen-webviewrpc=./protoc-gen-webviewrpc \
  --webviewrpc_out=cs_client,cs_server,js_client,js_server:./All \
  -I. my_service.proto
```