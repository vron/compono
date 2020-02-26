sha224-d14a028c2a3a2bc9476102bb288234c415a2b01f828ea62ac5b3e42f
sha224-d14a028c2a3a2bc9476102bb288234c415a2b01f828ea62ac5b3e42f-1000345-schema

errors should be visible in gui - maybe also some logging, so send that.

blob-storage:

- should not (normally) duplicate (threshold? on size or no?)
- should store stuff continious if likely to be accessed continuous.
- cache should be more likely when they are shared or stored not consequtive?

* What if we assume that the order it is uploaded is likely to roughly be the order in which it is downloaded? - That makes sense right? - at least per client!

* We should definitely to send an 200 OK back before we have flushed to disk and whatever else we want to do! (what about backing up to other places? - liekly needs to be configurable if we should wait for or async to)

* what about reading the data back and hashing? could help for bad ram errors? - they are actually caught since the client has hashed! Assuming we do not copy around after having checked the hash!!

TODO: PID lock file to ensure that only one process at the time is reading it

Additionally we keep a index listing all the blobs we have

Some size considerations to remember:

Saved data: 10 TB
Av blob size: 500 kB

No blobs for that: 20 e6
Size just blob list: 560 Mb
Size blob list and sizes: 640 MB

DID NOT LIKE:
no diff between schema non schema - security hole as backup e.g. could creare a share?

0
