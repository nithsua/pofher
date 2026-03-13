# gopho

A bare-metal x86 OS written in Go.

## Design Notes

### Ring Based Security

- Ring 3 => User land
- Ring 0 => Kernel

### Bootloader

GRUB2 is the choice of bootloader to keep things simple.

- Kernel must define a special header that begins with a magic value
- Header must be defined in the first 32KiB (Multiboot2) of the kernel image
- GRUB2 will load the kernel from the specified location

### Linker

The linker needs to be told where to place each section of the binary in memory.
We will be using GNU ld as the linker.

```
+-----------------------+
|         0x00          |  Low Memory
+-----------------------+
|          ...          |
|-----------------------|---- 1 MB (Boundary)
|///////////////////////|
|//// Kernel Image /////|  _rt0_entry
|///////////////////////|
+-----------------------+
|          ...          |  High Memory
+-----------------------+
```

### Bootstrap Requirements

After GRUB hands control to the kernel entry point, several things are missing before Go can run:

**Stack**
Go doesn't work without a stack. We bootstrap one by:
- Reserving 16K of uninitialized data (`.bss` section)
- Loading the stack pointer with the address of the end of this block

**SSE (SIMD Extensions)**
SSE is disabled on boot. Go's compiler emits SSE instructions (vectorization). Without enabling SSE, the CPU throws a fatal exception (`#UD`). SSE must be enabled via CR0/CR4 before jumping to Go.

**Memory Models**

Flat Memory Model — every pointer is a direct physical address:
```
0x00 [   ]
0x01 [   ]
0x02 [   ]       +-----------------+
0x03 [X] <-------|    (uintptr)    |
0x04 [   ]       +-----------------+
 ...
4Gb  [   ]
```

Segmented Addressing — the `gs` segment register holds a base address; effective addresses are `gs_base + offset`:
```
gs (Segment Base) --> 0x01
Effective Address: gs:0x02 --> 0x03
```

**Go's Stack Growth Precode**

Before every function call, Go emits a stack growth check:
```
if SP < g.stackguard0 {
    runtime.GrowStack()
}
```

The goroutine descriptor (`g`) is found via the `gs` segment register → Thread Control Block → current `g`:
```
gs:0x00 -> TCB -> g {
    stack.lo    (0x00)
    stack.hi    (0x04)
    stackguard0 (0x08)
}
```

We must:
- Set up the `gs` register (requires a custom GDT with a TLS segment descriptor)
- Populate `runtime.g0` (`stack.lo`, `stack.hi`, `stackguard0 = stack.lo` to bypass growth checks)

### Build Process

Cross-compile targeting 386:
```
GOARCH=386 GOOS=linux go build -n
```

The `-n` flag outputs the build script. We intercept it to inject:
- `-linkmode=external`
- `-extldflags='-nostdlib'`

Then link with GNU ld:
```
# Make runtime.g0 symbol public
objcopy --globalize-symbol runtime.g0 go.o

# Link boot assembly + Go object into final ELF
ld -T linker.ld -o gopho.elf boot.o go.o

# Create bootable ISO
grub-mkrescue -o gopho.iso iso/
```

### Screen Output

On boot, output is via VGA 80x25 text mode:
- Linear framebuffer at `0xB8000`
- 2 bytes per character: `[ascii byte][attribute byte]`
- Attribute byte: 4 bits background color, 4 bits foreground color (16 possible colors)

### Limitations

Unsupported Go runtime features:
- Maps
- Goroutines
- Interfaces
- `defer`
- Memory allocator (calls to allocator will cause triple-fault)
