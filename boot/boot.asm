; Copyright (c) 2026 Nivas Muthu M G. All rights reserved.
; Use of this source code is governed by a GPL v2 license
; that can be found in the LICENSE file.

; Why Assembly?
; Go cannot run on bare metal without a valid stack, working gs register and SSE enabled.
; Which would require direct CPU manipulation which go doesn't inherently provide syntax for.
;
; Global Descriptor Table:
; The GDT defined memory segments. In 32-bit protected mode. every memory access goes through
; a segment register(cs, ds, es, fs, gs, ss). Each segment holds a selector, an index into the
; GDT and the CPU looks up that entry to get the base address, limit and permissions for that
; segment.
;
; Since the GDT provided by GRUB is very minimal we will be implementing our own GDT.
;
; Entry 0: Null Descriptor (As per spec, first entry always tends to be null)
; Entry 1: Code Segment(cs) base 0x0, limit 4GB, executable, ring 0
; Entry 2: Data Segment(ds) base 0x0, limit 4GB, writable, ring 0
; Entry 3: TLS segment() base = address of the TLS block, limit = Size of the TLS block
;
; Each GDT Entry is 8 byte with complex layout
; Bits 0-15: Limit (low 16 bits)
; Bits 16-39: Base (low 24 bits)
; Bit 40: Accessed
; Bit 41: Read/writable
; Bit 42: Direction/Conforming
; Bit 43: Executable
; Bit 44: Descriptor type (1 = Code/Data)
; Bis 45-46: Privilege level (0 = Ring 0)
; Bit 47: Present (must be 1)
; Bits 48-51: Limit (high 4 bits)
; Bits 52-53; Available / Long mode flag
; Bit 54: Size (1 = 32 bit, 0 = 16 bit)
; Bit 55: Granularity (1 = limit in 4k pages, 0 = limit in bytes)
; Bits 56-63: Base (high 8 bits)
;
; GDT will be loaded using lgdt instruction which will take a GDTR which is a 6 byte structure
; containing the GDT's size(16 bit) and linear address(32 bit).
;
; After lgdt. All the segment registers should be loaded. ds, es, fs, ss are loaded using
; mov instruction. cs cannot be loaded directly - it needs to be reloaded with a far jump.
;
; Enabling SSE
; Go compiler emits SSE instructions. Without enabling SSE. CPU will raise #UD(Invalid Opcode)
; exception on encountering one, Which without an IDT causes a triple fault.
;
; We can enable SSE by using 2 control registers - CR0 and CR4
;
; CR0:
; Clear bit 2 (EM - Emulation). This bit tells the CPU tto emulate x97/FPU in software.
; If set, SSE raises #UD.
; Set bit 1 (MP - Monitor Coprocessor). This enables the WAIT/FWAIT instruction to check for
; pending FPU exceptions
;
; CR4:
; Set bit 0(OSFXSR). Tells the CPU the OS supports FXSAVE/FXRSTOR and SSE state saving.
; Set bit 10(OSXMMEXCPT). Tells the CPU to raise #XF (SIMD exception) instead of #UD on SSE errors
;
; Control registers can't be written to directly. they will have to be mov to a
; general purpose register modified and then mov back.
;
; Stub IDT
; An Interrupt Descriptor Table tells the CPU what to do when an exception or interrupt fires
