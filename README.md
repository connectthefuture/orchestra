A 5 Minute Guide to Orchestra
-----------------------------

What is it?
===========

Orchestra is a series of tools for Getting Shit Run.

It consists of a Conductor, which is the coordinating process, and
Players, which are the actual daemons running on nodes to do the work.

To prevent arbitrary execution of code, Players can only execute
predefined scores which have to be installed on them separately.  You
can use Puppet, CFEngine or some other configuration management system
to do this.

Canonically, entities requesting work to be done are known as the
Audience.

Please read the Orchestra paper (in doc/) for more information.


License
=======

Copyright (c) 2011-2015 Anchor Systems Pty Ltd
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

 * Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.
 * Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the distribution.
 * Neither the name of Anchor Systems Pty Ltd nor the
   names of its contributors may be used to endorse or promote products
   derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL ANCHOR SYSTEMS PTY LTD BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.


Building
========

To build Orchestra you will need:

 * The [Go](http://golang.org/) compiler. Orchestra has been tested on
   version 1.4.2; older versions may or may not work.
 * The [Protocol Buffers](https://developers.google.com/protocol-buffers/)
   compiler, `protoc`.
 * The [Go language Protocol Buffers compiler plugin](https://github.com/golang/protobuf),
   `protoc-gen-go`.
 * Some Go libraries (in your `GOPATH`):
   * [`github.com/kuroneko/configureit`](https://github.com/kuroneko/configureit)
   * [`github.com/golang/protobuf/proto`](https://github.com/golang/protobuf)
 * [GNU Make](https://www.gnu.org/software/make/).

From the top-level directory, run `gmake`. The Orchestra binaries will be
installed into `bin/`.


Source Layout
=============

 * `src/`     -- All the Go sources for the conductor, player, and the
                 `submitjob` and `getstatus` sample implementations.
 * `doc/`     -- Documentation about Orchestra and its implementation.
 * `samples/` -- Sample configuration files.
 * `python/`  -- Python client library for communicating with the
                 conductor as the audience.


Known Issues
============

 * There is no clean up of job data or persistence of results at this
   time.

 * `getstatus` gets back a lot more information than it displays.

 * No efficient 'wait for job' interface yet.  You have to poll the
   audience interface for completion results for now.  (The polls are,
   however, stupidly cheap.)

 * Disconnect/reconnect behaviour for players is not particularly well
   tested.  Anecdotal evidence suggests that this is relatively
   robust however.

 * Jobs will be left dangling if a player is removed from the
   conductor's configuration and the conductor HUP'd whilst there is
   still work pending for that player.

 * Some of the more advanced score scheduling ideas that we've had
   remain unimplemented, resulting in Orchestra looking a lot blander
   than it really is meant to be.

 * There is no support for CRLs yet.
