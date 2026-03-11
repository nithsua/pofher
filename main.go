/*
Ring Based Security
-> Ring 3 => User land
-> Ring 1,2 =>
-> Ring 0 => Kernel land
*/

/*
Booloader
GRUB2 is the choice of Bootloader to keep things simple
-> Kernel must define a special header that begins with a magic value
-> Header must be defined in the first 4K of the kernel image
-> GRUB2 will load the kernel from the specified location
*/

/*
Linker
the linker needs to be specified where to place each section of the binary in memory
We will be using GNU ld as the linker

Eg.
ENTRY(_rt0_entry)
SECTIONS {
    .=1M;
	.multiboot :{ *(.multiboot_header) }
	/ executable code /
	.text BLOCK(4K) : ALIGN(4K) { *(.text) }
	/ read-only data
	.rodata BLOCK(4K) : ALIGN(4K) { *(.rodata) }
	/ read-write data (initialized)
	.data BLOCK(4K) : ALIGN(4K) { *(.data) }
	/ read-wtire data (uninitialized)
	.bss BLOCK(4K) : ALIGN(4K) { *(COMMON) *(.bss) }
}

	+-----------------------+
    |         0x00          |  Low Memory
    +-----------------------+
    |          ...          |
----|-----------------------|---- 1 Mb (Boundary)
    |///////////////////////|
    |//// Kernel Image /////|  (Gray shaded area) _rt0_entry
    |///////////////////////|
    +-----------------------+
    |          ...          |  High Memory
    +-----------------------+
*/

/*
What's up after
-> There is no stack. Go doesn't work without a stack
-> Streaming SSE(SIMD Extemsions) are disabled.
	=> Single Instruction, Multiple Data
	=> Allows us to perform an operation to multiple values concurrently
	=> E.g.
		=> When we a slice of 8 integers and we want to increment all the values by . Traditionally we
		loop through and increment the value and the same would be translated to assembly in a naive compiler.
		=> Unlike that Go compiler does optimization(vectorization) and instead of translating to 8
		instructions it translates to 1 or 2 SIMD instructions which operates on 4 or 8 elements concurrently
	=> So ideally a support for this should be implemented this otherwise the CPU will through a fatal exception
-> Memory
	=> Flat Memory Model
	=> Segmented Addressing Memory Model

	Flat Memory Model
      +-----------------+
 0x00 |      [   ]      |
      +-----------------+
 0x01 |      [   ]      |
      +-----------------+
 0x02 |      [   ]      |       +-----------------+
      +-----------------+       |     pointer     |
 0x03 |      [X] <--------------|    (uintptr)    |
      +-----------------+       +-----------------+
 0x04 |      [   ]      |
      +-----------------+
              ...
      +-----------------+
  4Gb |      [   ]      |
      +-----------------+

	  Segmented Addressing
      +-----------------+
 0x00 |      [   ]      |
      +-----------------+       +-----------------+
 0x01 |      [   ] <------------|       gs        | (Segment Base)
      +---------^-------+       +-----------------+
 0x02 |      [  |  ]    |                |
      +---------|-------+                |
 0x03 |      [  X  ] <----------+        | Offset: 0x02
      +---------|-------+       |        |
 0x04 |      [  |  ]    |       |        |
      +---------v-------+       |  +------------+
              ...               +--|  gs:0x02   | (Effective Address)
      +-----------------+          +------------+
  4Gb |      [   ]      |                ^
      +-----------------+                |
                                 (Segment Register)


	=> When a go func runs. a pre code(Stack Growth Check) is executed before the actual function.
	=> Code calls foo()
	=> E.g.
		Code calls foo()
			- Feel ech pointer to current g
			- If SP < g.stackguard0 {
				runtime.GrowStack()
			}
			- Jump to foo() code

		$GOROOT/src/runtime/runtime2.go
		type g struct {
			stack 			stack
			stackguard0 	uintptr
			...
		}

		type stack struct {
			lo uintptr
			hi uintptr
		}
	=> Execution flow
		func foo() {
			print("Hello")
		}

		After build, GOOS=linux GOARCH=386 go build
		objdump -d (Intel x86 syntax)

		```
		0808aae0 <main.foo>:
		01 mov ecx,DWORD PTR gs:0x0  # The code uses the gs segment register to find the thread-local storage.
		02 mov ecx,DWORD PTR [ecx-0x4] # It follows a pointer to the Thread Control Block (TCB), which leads to the Current g (the current goroutine's descriptor).
		02 cmp esp,DWORD PTR [ecx-0x8] # It compares the stack pointer (esp) with the stack guard (g.stackguard0).
		04 jbe 909ab19 (grow stack) # If the stack pointer is less than the stack guard, it calls the GrowStack function to grow the stack.
		```

	gs: 0x00 -> Ptr to ? ->
	Thread Control Block [TCB - 0x04] Ptr to g ->
	+--------------------+
	|		Current g    |
	+--------------------+
	| 0x00 | stack.lo    |
	+--------------------+
	| 0x04 | stack.hi    |
	+--------------------+
	| 0x08 | stackguard0 |
	+--------------------+

-> Precode stack checker happens but the stack itself is missing since it's running on bare metal. So we will bootstrap a stack
	=> Reserve 16K of uninitialized data (.bss section) for kernel image
		- Load stack pointer with address to end of this block
	=> Setup gs register according to the Application Binary Interface for Thread Local Storage
	=> Populate a g struct
		- Runtime package already initializes a g instance for the main goroutine called g0
		- Set g0.stack.hi / g0.stack.lo to bounds of the reserved stack memory
		- Set g0.stackguard0 = g0.stack.lo -> bypass stack growth checks

-> Go Build
	=> We can target 386 using go build's cross compilation
	=> A seperate link step has to be performed to link the object files into a binary
		-> This is done by intercepting go build process
		-> -n flag of go build will output the build script used for the package
			GOARCH=386 GOOS=linux go build -n 2>&1 | sed -e "ls|^|set -e \n|" -e "ls|^|export GOOS=linux \n|" -e "ls|^|export GOARCH=386 \n|" \
				-e "ls|^|WORK='$(BUILD_ABS_DIR)\n'" -e "s|-extld|-linkmode=external -tmpdir='$(BUILD_ABS_DIR)' -extldflags='-nostdlib' -extld|g" | bash
-> Linking the final kernel image
	=> Use GNU ld to link the final kernel image
		- The object file produced by nasm
		- The go.o file produced by the modified go build step
	=> Invoking the linker will ideally throw error since all symbols in go object files are private
	=> objcopy can be used to make symbols public
		- objcopy --globalize-symbols runtime.g0 $(BUILD_DIR)/go.o $(BUILD_DIR)/go.o

-> Screen output
	=> Now on boot we will display to the screen by spinning up in VGA 80x25 text mode
		- Linear framebuffer located at 0xB8000
	=> 2 bytes per character. One for the character code and one for the color attributes
	=> Attribute byte
		- 4 bits for background color, 4 bits for foreground color.
		- 16 possible colors (2^4)
-> Limitations
	=> Unsupported go runtime features
		- Maps
		- Goroutines
		- Interfaces
		- defer
	=> Go memory allocator
		- Calls to allocator will cause triple-fault
*/

package main

func main() {

}
