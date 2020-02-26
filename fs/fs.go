package fs

import (
	"errors"

	"github.com/billziss-gh/cgofuse/fuse"
)

type FileSystem struct {
	fuse.FileSystemBase

	shouldLog bool
}

func Mount(path string, shouldLog bool) error {
	f := &FileSystem{shouldLog: shouldLog}
	host := fuse.NewFileSystemHost(f)
	if !host.Mount(path, []string{"-d"}) {
		return errors.New("aaa")
	}
	return nil
}

/*


func split(path string) []string {
	return strings.Split(path, "/")
}

func resize(slice []byte, size int64, zeroinit bool) []byte {
	const allocunit = 64 * 1024
	allocsize := (size + allocunit - 1) / allocunit * allocunit
	if cap(slice) != int(allocsize) {
		var newslice []byte
		{
			defer func() {
				if r := recover(); nil != r {
					panic(fuse.Error(-fuse.ENOSPC))
				}
			}()
			newslice = make([]byte, size, allocsize)
		}
		copy(newslice, slice)
		slice = newslice
	} else if zeroinit {
		i := len(slice)
		slice = slice[:size]
		for ; len(slice) > i; i++ {
			slice[i] = 0
		}
	}
	return slice
}
*/

/*

type node_t struct {
	stat    fuse.Stat_t
	xatr    map[string][]byte
	chld    map[string]*node_t
	data    []byte
	opencnt int
}

func newNode(dev uint64, ino uint64, mode uint32, uid uint32, gid uint32) *node_t {
	tmsp := fuse.Now()
	self := node_t{
		fuse.Stat_t{
			Dev:      dev,
			Ino:      ino,
			Mode:     mode,
			Nlink:    1,
			Uid:      uid,
			Gid:      gid,
			Atim:     tmsp,
			Mtim:     tmsp,
			Ctim:     tmsp,
			Birthtim: tmsp,
			Flags:    0,
		},
		nil,
		nil,
		nil,
		0}
	if fuse.S_IFDIR == fs.stat.Mode&fuse.S_IFMT {
		fs.chld = map[string]*node_t{}
	}
	return &self
}

type Memfs struct {
	fuse.FileSystemBase
	lock    sync.Mutex
	ino     uint64
	root    *node_t
	openmap map[uint64]*node_t
}
*/

func (fs *FileSystem) Mknod(path string, mode uint32, dev uint64) (errc int) {
	if fs.shouldLog {
		defer fs.trace(path, mode, dev)(&errc)
	}

	defer fs.synchronize()()
	return fs.makeNode(path, mode, dev, nil)
}

func (fs *FileSystem) Mkdir(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	defer fs.synchronize()()
	return fs.makeNode(path, fuse.S_IFDIR|(mode&07777), 0, nil)
}

func (fs *FileSystem) Unlink(path string) (errc int) {
	defer trace(path)(&errc)
	defer fs.synchronize()()
	return fs.removeNode(path, false)
}

func (fs *FileSystem) Rmdir(path string) (errc int) {
	defer trace(path)(&errc)
	defer fs.synchronize()()
	return fs.removeNode(path, true)
}

func (fs *FileSystem) Link(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer fs.synchronize()()
	_, _, oldnode := fs.lookupNode(oldpath, nil)
	if nil == oldnode {
		return -fuse.ENOENT
	}
	newprnt, newname, newnode := fs.lookupNode(newpath, nil)
	if nil == newprnt {
		return -fuse.ENOENT
	}
	if nil != newnode {
		return -fuse.EEXIST
	}
	oldnode.stat.Nlink++
	newprnt.chld[newname] = oldnode
	tmsp := fuse.Now()
	oldnode.stat.Ctim = tmsp
	newprnt.stat.Ctim = tmsp
	newprnt.stat.Mtim = tmsp
	return 0
}

func (fs *FileSystem) Symlink(target string, newpath string) (errc int) {
	defer trace(target, newpath)(&errc)
	defer fs.synchronize()()
	return fs.makeNode(newpath, fuse.S_IFLNK|00777, 0, []byte(target))
}

func (fs *FileSystem) Readlink(path string) (errc int, target string) {
	defer trace(path)(&errc, &target)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT, ""
	}
	if fuse.S_IFLNK != node.stat.Mode&fuse.S_IFMT {
		return -fuse.EINVAL, ""
	}
	return 0, string(node.data)
}

func (fs *FileSystem) Rename(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer fs.synchronize()()
	oldprnt, oldname, oldnode := fs.lookupNode(oldpath, nil)
	if nil == oldnode {
		return -fuse.ENOENT
	}
	newprnt, newname, newnode := fs.lookupNode(newpath, oldnode)
	if nil == newprnt {
		return -fuse.ENOENT
	}
	if "" == newname {
		// guard against directory loop creation
		return -fuse.EINVAL
	}
	if oldprnt == newprnt && oldname == newname {
		return 0
	}
	if nil != newnode {
		errc = fs.removeNode(newpath, fuse.S_IFDIR == oldnode.stat.Mode&fuse.S_IFMT)
		if 0 != errc {
			return errc
		}
	}
	delete(oldprnt.chld, oldname)
	newprnt.chld[newname] = oldnode
	return 0
}

func (fs *FileSystem) Chmod(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	node.stat.Mode = (node.stat.Mode & fuse.S_IFMT) | mode&07777
	node.stat.Ctim = fuse.Now()
	return 0
}

func (fs *FileSystem) Chown(path string, uid uint32, gid uint32) (errc int) {
	defer trace(path, uid, gid)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	if ^uint32(0) != uid {
		node.stat.Uid = uid
	}
	if ^uint32(0) != gid {
		node.stat.Gid = gid
	}
	node.stat.Ctim = fuse.Now()
	return 0
}

func (fs *FileSystem) Utimens(path string, tmsp []fuse.Timespec) (errc int) {
	defer trace(path, tmsp)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	node.stat.Ctim = fuse.Now()
	if nil == tmsp {
		tmsp0 := node.stat.Ctim
		tmsa := [2]fuse.Timespec{tmsp0, tmsp0}
		tmsp = tmsa[:]
	}
	node.stat.Atim = tmsp[0]
	node.stat.Mtim = tmsp[1]
	return 0
}

func (fs *FileSystem) Open(path string, flags int) (errc int, fh uint64) {
	defer trace(path, flags)(&errc, &fh)
	defer fs.synchronize()()
	return fs.openNode(path, false)
}

func (fs *FileSystem) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer trace(path, fh)(&errc, stat)
	defer fs.synchronize()()
	node := fs.getNode(path, fh)
	if nil == node {
		return -fuse.ENOENT
	}
	*stat = node.stat
	return 0
}

func (fs *FileSystem) Truncate(path string, size int64, fh uint64) (errc int) {
	defer trace(path, size, fh)(&errc)
	defer fs.synchronize()()
	node := fs.getNode(path, fh)
	if nil == node {
		return -fuse.ENOENT
	}
	node.data = resize(node.data, size, true)
	node.stat.Size = size
	tmsp := fuse.Now()
	node.stat.Ctim = tmsp
	node.stat.Mtim = tmsp
	return 0
}

func (fs *FileSystem) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	defer fs.synchronize()()
	node := fs.getNode(path, fh)
	if nil == node {
		return -fuse.ENOENT
	}
	endofst := ofst + int64(len(buff))
	if endofst > node.stat.Size {
		endofst = node.stat.Size
	}
	if endofst < ofst {
		return 0
	}
	n = copy(buff, node.data[ofst:endofst])
	node.stat.Atim = fuse.Now()
	return
}

func (fs *FileSystem) Write(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	defer fs.synchronize()()
	node := fs.getNode(path, fh)
	if nil == node {
		return -fuse.ENOENT
	}
	endofst := ofst + int64(len(buff))
	if endofst > node.stat.Size {
		node.data = resize(node.data, endofst, true)
		node.stat.Size = endofst
	}
	n = copy(node.data[ofst:endofst], buff)
	tmsp := fuse.Now()
	node.stat.Ctim = tmsp
	node.stat.Mtim = tmsp
	return
}

func (fs *FileSystem) Release(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	defer fs.synchronize()()
	return fs.closeNode(fh)
}

func (fs *FileSystem) Opendir(path string) (errc int, fh uint64) {
	defer trace(path)(&errc, &fh)
	defer fs.synchronize()()
	return fs.openNode(path, true)
}

func (fs *FileSystem) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	defer trace(path, fill, ofst, fh)(&errc)
	defer fs.synchronize()()
	node := fs.openmap[fh]
	fill(".", &node.stat, 0)
	fill("..", nil, 0)
	for name, chld := range node.chld {
		if !fill(name, &chld.stat, 0) {
			break
		}
	}
	return 0
}

func (fs *FileSystem) Releasedir(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	defer fs.synchronize()()
	return fs.closeNode(fh)
}

func (fs *FileSystem) Setxattr(path string, name string, value []byte, flags int) (errc int) {
	defer trace(path, name, value, flags)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	if "com.apple.ResourceFork" == name {
		return -fuse.ENOTSUP
	}
	if fuse.XATTR_CREATE == flags {
		if _, ok := node.xatr[name]; ok {
			return -fuse.EEXIST
		}
	} else if fuse.XATTR_REPLACE == flags {
		if _, ok := node.xatr[name]; !ok {
			return -fuse.ENOATTR
		}
	}
	xatr := make([]byte, len(value))
	copy(xatr, value)
	if nil == node.xatr {
		node.xatr = map[string][]byte{}
	}
	node.xatr[name] = xatr
	return 0
}

func (fs *FileSystem) Getxattr(path string, name string) (errc int, xatr []byte) {
	defer trace(path, name)(&errc, &xatr)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT, nil
	}
	if "com.apple.ResourceFork" == name {
		return -fuse.ENOTSUP, nil
	}
	xatr, ok := node.xatr[name]
	if !ok {
		return -fuse.ENOATTR, nil
	}
	return 0, xatr
}

func (fs *FileSystem) Removexattr(path string, name string) (errc int) {
	defer trace(path, name)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	if "com.apple.ResourceFork" == name {
		return -fuse.ENOTSUP
	}
	if _, ok := node.xatr[name]; !ok {
		return -fuse.ENOATTR
	}
	delete(node.xatr, name)
	return 0
}

func (fs *FileSystem) Listxattr(path string, fill func(name string) bool) (errc int) {
	defer trace(path, fill)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	for name := range node.xatr {
		if !fill(name) {
			return -fuse.ERANGE
		}
	}
	return 0
}

func (fs *FileSystem) Chflags(path string, flags uint32) (errc int) {
	defer trace(path, flags)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	node.stat.Flags = flags
	node.stat.Ctim = fuse.Now()
	return 0
}

func (fs *FileSystem) Setcrtime(path string, tmsp fuse.Timespec) (errc int) {
	defer trace(path, tmsp)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	node.stat.Birthtim = tmsp
	node.stat.Ctim = fuse.Now()
	return 0
}

func (fs *FileSystem) Setchgtime(path string, tmsp fuse.Timespec) (errc int) {
	defer trace(path, tmsp)(&errc)
	defer fs.synchronize()()
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	node.stat.Ctim = tmsp
	return 0
}

func (fs *FileSystem) lookupNode(path string, ancestor *node_t) (prnt *node_t, name string, node *node_t) {
	prnt = fs.root
	name = ""
	node = fs.root
	for _, c := range split(path) {
		if "" != c {
			if 255 < len(c) {
				panic(fuse.Error(-fuse.ENAMETOOLONG))
			}
			prnt, name = node, c
			if node == nil {
				return
			}
			node = node.chld[c]
			if nil != ancestor && node == ancestor {
				name = "" // special case loop condition
				return
			}
		}
	}
	return
}

func (fs *FileSystem) makeNode(path string, mode uint32, dev uint64, data []byte) int {
	prnt, name, node := fs.lookupNode(path, nil)
	if nil == prnt {
		return -fuse.ENOENT
	}
	if nil != node {
		return -fuse.EEXIST
	}
	fs.ino++
	uid, gid, _ := fuse.Getcontext()
	node = newNode(dev, fs.ino, mode, uid, gid)
	if nil != data {
		node.data = make([]byte, len(data))
		node.stat.Size = int64(len(data))
		copy(node.data, data)
	}
	prnt.chld[name] = node
	prnt.stat.Ctim = node.stat.Ctim
	prnt.stat.Mtim = node.stat.Ctim
	return 0
}

func (fs *FileSystem) removeNode(path string, dir bool) int {
	prnt, name, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT
	}
	if !dir && fuse.S_IFDIR == node.stat.Mode&fuse.S_IFMT {
		return -fuse.EISDIR
	}
	if dir && fuse.S_IFDIR != node.stat.Mode&fuse.S_IFMT {
		return -fuse.ENOTDIR
	}
	if 0 < len(node.chld) {
		return -fuse.ENOTEMPTY
	}
	node.stat.Nlink--
	delete(prnt.chld, name)
	tmsp := fuse.Now()
	node.stat.Ctim = tmsp
	prnt.stat.Ctim = tmsp
	prnt.stat.Mtim = tmsp
	return 0
}

func (fs *FileSystem) openNode(path string, dir bool) (int, uint64) {
	_, _, node := fs.lookupNode(path, nil)
	if nil == node {
		return -fuse.ENOENT, ^uint64(0)
	}
	if !dir && fuse.S_IFDIR == node.stat.Mode&fuse.S_IFMT {
		return -fuse.EISDIR, ^uint64(0)
	}
	if dir && fuse.S_IFDIR != node.stat.Mode&fuse.S_IFMT {
		return -fuse.ENOTDIR, ^uint64(0)
	}
	node.opencnt++
	if 1 == node.opencnt {
		fs.openmap[node.stat.Ino] = node
	}
	return 0, node.stat.Ino
}

func (fs *FileSystem) closeNode(fh uint64) int {
	node := fs.openmap[fh]
	node.opencnt--
	if 0 == node.opencnt {
		delete(fs.openmap, node.stat.Ino)
	}
	return 0
}

func (fs *FileSystem) getNode(path string, fh uint64) *node_t {
	if ^uint64(0) == fh {
		_, _, node := fs.lookupNode(path, nil)
		return node
	} else {
		return fs.openmap[fh]
	}
}

func NewMemfs() *Memfs {
	self := Memfs{}
	defer fs.synchronize()()
	fs.ino++
	fs.root = newNode(0, fs.ino, fuse.S_IFDIR|00777, 0, 0)
	fs.openmap = map[uint64]*node_t{}
	return &self
}
