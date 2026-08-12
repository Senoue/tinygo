// Startup code for the ESP32-C5.
//
// The ESP32-C5 TinyGo port uses a RAM-only image: the ROM bootloader loads
// all segments (.data, .text, .rodata) into HP-SRAM before jumping here, so
// no flash cache/MMU setup is needed — unlike the ESP32-C3/C6 ports.
// (The C5 eco3 ROM crashes when the image at flash offset 0x2000 contains
// flash-mapped segments, so everything runs from SRAM.)

.section .init
.global call_start_cpu0
.type call_start_cpu0,@function
call_start_cpu0:
    // At this point:
    // - The ROM bootloader is finished and has jumped to here.
    // - All image segments have been loaded into SRAM.
    // - We have a usable stack (but not the one we would like to use).

    // Jump to generic RISC-V initialization, which initializes the stack
    // pointer and globals register. It should not return.
    j _start

// Trap entry point for the CLIC (in non-vectored mode).
//
// The ESP32-C5 uses a CLIC as its CPU interrupt controller. All interrupts
// are configured as non-vectored (shv=0), so every trap — exceptions and
// interrupts alike — enters at the mtvec base address, which must be 64-byte
// aligned in CLIC mode (mtvec[31:6] << 6). The interrupt cause is read from
// mcause by the Go dispatcher (handleInterrupt).
.section .text.exception_vectors
.global _vector_table
.type _vector_table,@function

.balign 64
_vector_table:

    .option push
    .option norvc

    .rept 48
    j handleInterruptASM            /* exception / interrupt handler */
    .endr

    .option pop

.size _vector_table, .-_vector_table

// CLIC hardware vector table (mtvt, CSR 0x307).
//
// Unlike mtvec, the mtvt table contains function POINTERS (one .word per
// CLIC line), not jump instructions. The Espressif CLIC fetches the handler
// address from mtvt + 4*line when delivering a hardware-vectored interrupt,
// so this table must be valid. All 48 lines point at the common assembly
// handler; the Go dispatcher reads mcause to find the actual line.
.global _mtvt_table
.type _mtvt_table,@object

.balign 64
_mtvt_table:
    .rept 48
    .word handleInterruptASM
    .endr

.size _mtvt_table, .-_mtvt_table
