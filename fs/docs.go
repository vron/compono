package trace

/*

want it to:

1. be fast..
	-> write locally and sync remote async

2. be dependable	
	-> since we wrie remotely async they can never fail!
		e.g. 2 offline writes

3. offline
	-> make specific folders available offline


client needs to listen to updates on particular paths (those offline - or cache? open?)






Create:
func (*FileSystemBase) Create(path string, flags int, mode uint32) (int, uint64)
func (*FileSystemBase) Link(oldpath string, newpath string) int
func (*FileSystemBase) Mkdir(path string, mode uint32) int
func (*FileSystemBase) Mknod(path string, mode uint32, dev uint64) int
func (*FileSystemBase) Symlink(target string, newpath string) int


Modify:
func (*FileSystemBase) Chmod(path string, mode uint32) int
func (*FileSystemBase) Chown(path string, uid uint32, gid uint32) int
func (*FileSystemBase) Removexattr(path string, name string) int
func (*FileSystemBase) Rename(oldpath string, newpath string) int
func (*FileSystemBase) Rmdir(path string) int
func (*FileSystemBase) Setxattr(path string, name string, value []byte, flags int) int
func (*FileSystemBase) Truncate(path string, size int64, fh uint64) int
func (*FileSystemBase) Unlink(path string) int
func (*FileSystemBase) Write(path string, buff []byte, ofst int64, fh uint64) int


Flush:
func (*FileSystemBase) Flush(path string, fh uint64) int
func (*FileSystemBase) Fsync(path string, datasync bool, fh uint64) int
func (*FileSystemBase) Fsyncdir(path string, datasync bool, fh uint64) int


Read:
func (*FileSystemBase) Access(path string, mask uint32) int
func (*FileSystemBase) Getattr(path string, stat *Stat_t, fh uint64) int
func (*FileSystemBase) Getxattr(path string, name string) (int, []byte)
func (*FileSystemBase) Listxattr(path string, fill func(name string) bool) int
func (*FileSystemBase) Open(path string, flags int) (int, uint64)
func (*FileSystemBase) Opendir(path string) (int, uint64)
func (*FileSystemBase) Read(path string, buff []byte, ofst int64, fh uint64) int
func (*FileSystemBase) Readdir(path string, fill func(name string, stat *Stat_t, ofst int64) bool, ofst int64, fh uint64) int
func (*FileSystemBase) Readlink(path string) (int, string)
func (*FileSystemBase) Statfs(path string, stat *Statfs_t) int


Admin:
func (*FileSystemBase) Destroy()
func (*FileSystemBase) Init()
func (*FileSystemBase) Release(path string, fh uint64) int
func (*FileSystemBase) Releasedir(path string, fh uint64) int
func (*FileSystemBase) Utimens(path string, tmsp []Timespec) int

*/