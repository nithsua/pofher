// Copyright (c) 2026 Nivas Muthu M G. All rights reserved.
// Use of this source code is governed by a GPL v2 license
// that can be found in the LICENSE file.

/*
We will be placing the binary from 1MB of the physical memory
1MB because from 0 to 1MB the physcial memory would look like below in a x86 system:

0x00000000 - 0x000004ff BIOS interrupt vector table + BIOS data area
0x00007C00 - 0x00007DFF Bootloader
0x00080000 - 0x0009FFFF BIOS extended data area
0x000A0000 - 0x000BFFFF Kernel VGA Memory (0xB8000 is text mode here)
0x000C0000 - 0x000FFFFF BIOS ROM,  hardware reserved
----------------------------------------------------------------------1 MB boundary
0x00100000 - usable memory bound starts here
*/
ENTRY(_rt0_entry) /* Entry point symbol. The very first instruction the CPU will execute when kernel is jumped to */
SECTIONS {
    /* Do not touch the first 1MiB of memory */
    . = 1M;
    /*
    GRUB scans the first 32KiB of the kernel image looking for Multiboot2 magic number(0xE85250D6)
    If not found in the bound of the first 32KiB(of the usuable memory). GRUB will not load the kernel.

    *(.multiboot_header) corresponds to collect the .multiboot_header section from all input object and place it here.
    */
    .multiboot_header : { *(.multiboot_header) }
    /*
    BLOCK(4K) - start this section on a 4K page boundary.
    ALIGN(4K) - align the section's contents to 4K

    Aligning by 4K helps to later implement page-level permissions and memory management easier since pages are 4K bounded
    */
    .text BLOCK(4K) : ALIGN(4K) { *(.text) }
    /*
    String literal, constants. Seperate from .text so you can later mark it non-executable.
    */
    .rodata BLOCK(4K) : ALIGN(4K) { *(.rodata) }
    /*
    Data section. Variables, initialized global/static data.
    */
    .data BLOCK(4K) : ALIGN(4K) { *(.data) }
    /*
    Uninitialized data section. Variables, global/static data without initializers.
    The linker doesn't store these bytes in the binary.
    Bootloader/Kernel is responsoble for zeroing this section before use.

    This is where the 16K kernel stack will live - a block of .bss that you can reserve in the assembly file.

    *(COMMON) is a linker input section for tentative definitions (uninitialized globals in C). The comment is slightly misleading.
    */
    .bss BLOCK(4K) : ALIGN(4K) { *(COMMON) *(.bss) }
}

/*
Memory structure after GRUB loads the kernel.

0x00100000  <- . = 1M (start)
            [ .multiboot_header ]  <- GRUB finds the magic number here
            [ .text ]              <- _rt0_entry is here, page-aligned
            [ .rodata ]            <- constants, string, page-aligned
            [ .data ]              <- initialized globals, page-aligned
            [ .bss ]               <- zero'd globals -> 16k kernel stack
*/
