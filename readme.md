# My own HTTP/1.1 static web server

It's smol, it works, it's cute.

It's a little toy project for me to learn the fundamentals of HTTP and web servers, and learn Go while I'm at it.

You probably shouldn't be using this in any real capacity, certainly not in production environments.

## Features
- Static file routing and serving
- Keep-Alive
- ETag caching using FNV hash
- Editable text config
- gzip compression

## Quick start
```bash
git clone https://github.com/florian-rieder/http-server.git
cd http-server
go run .
```