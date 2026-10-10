#!/usr/bin/env python3
"""Read-only whole-source hashes and Git tree identity, excluding .git."""
import argparse, hashlib, os
from pathlib import Path

def obj(kind, data):
    return hashlib.sha1(kind.encode()+b' '+str(len(data)).encode()+b'\0'+data).digest()

def tree(path):
    items=[]
    for child in path.iterdir():
        if child.name=='.git':
            continue
        if child.is_symlink():
            mode=b'120000'; digest=obj('blob',os.readlink(child).encode()); key=child.name
        elif child.is_dir():
            mode=b'40000'; digest=tree(child); key=child.name+'/'
        else:
            mode=b'100755' if child.stat().st_mode&0o111 else b'100644'
            digest=obj('blob',child.read_bytes()); key=child.name
        items.append((key.encode(),mode+b' '+child.name.encode()+b'\0'+digest))
    return obj('tree',b''.join(value for _,value in sorted(items)))

if __name__=='__main__':
    p=argparse.ArgumentParser(); p.add_argument('root'); p.add_argument('--expected')
    args=p.parse_args(); digest=tree(Path(args.root)).hex(); print(digest)
    if args.expected and digest!=args.expected:
        raise SystemExit('Tree mismatch')
