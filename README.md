# grrsh: Go Reverse Remote Shell

This tool allows you to make reverse SSH connections. Essentially, it provides
a normal SSH server and client (with an optional proxy mode) where the TCP
client and server reverse roles. The client provides an SSH server that makes
outgoing connections to the server, which can either provide an SSH client or
proxy connections to a local port, allowing you to connect using a standard
OpenSSH client.

Many OpenSSH features are supported, including port forwarding, "netcat mode"
(-W), and even scp.

## Use Cases
* Network or "Internet of Things" devices where port forwarding is difficult or
  impractical.
* SSH servers on "road warriors" without a fixed address.

## License
```
Copyright (C) 2016-2018 mutantmonkey

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
```
