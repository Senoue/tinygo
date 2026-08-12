/* Linker script for the ESP32-C5
 *
 * This uses a RAM-only image layout: all segments (.data, .text, .rodata)
 * are loaded into HP-SRAM by the ROM bootloader with a plain memory copy.
 * Nothing runs from flash, so no cache-MMU setup is needed at startup.
 *
 * Rationale: the ESP32-C5 (eco3) ROM bootloader crashes with a load access
 * fault when the image at flash offset 0x2000 contains flash-mapped
 * (0x42000000-range) segments, unlike the C6 ROM which tolerates and skips
 * them. The ESP-IDF second-stage bootloader for the C5 is RAM-only as well,
 * so this is the only load path the ROM reliably supports.
 *
 * Memory facts:
 *   - 384kB of HP-SRAM at 0x40800000 (unified IRAM/DRAM address space).
 *   - The ROM bootloader uses the area from 0x4084E5A0 upward while loading
 *     (shared buffers, ROM stack, ROM .data/.bss), so loadable segments must
 *     stay below that boundary. After boot, the application reclaims the
 *     full 384kB (the heap extends to 0x40860000).
 *   - The image is flashed at offset 0x2000; since nothing is flash-mapped,
 *     no flash-offset/vaddr alignment tricks are needed.
 */

MEMORY
{
    /* Low HP-SRAM: never touched by the ROM loader. Used for NOLOAD
     * sections (stack, bss) and the heap.
     */
    RAM (rw) : ORIGIN = 0x40800000, LENGTH = 0x4BBA0

    /* ROM-loadable region: the same address range the ESP-IDF second-stage
     * bootloader is linked into (iram_seg + iram_loader_seg + dram_seg,
     * 0x4084BBA0..0x4085A5A0). Loading segments elsewhere (e.g. low SRAM)
     * makes the eco3 ROM loader crash.
     */
    LOADRAM (rwx) : ORIGIN = 0x4084BBA0, LENGTH = 0xEA00
}

/* The entry point. It is set in the image flashed to the chip, so must be
 * defined.
 */
ENTRY(call_start_cpu0)

SECTIONS
{
    /* Put the stack at the bottom of SRAM, so that the application will
     * crash on stack overflow instead of silently corrupting memory.
     * See: http://blog.japaric.io/stack-overflow-protection/
     */
    .stack (NOLOAD) :
    {
        . = ALIGN(16);
        . += _stack_size;
        _stack_top = .;
    } >RAM

    /* Global variables that are mutable and zero-initialized.
     * These must be zeroed at startup (unlike data, which is loaded by the
     * bootloader).
     */
    .bss (NOLOAD) : ALIGN(4)
    {
        . = ALIGN (4);
        _sbss = ABSOLUTE(.);
        *(.sbss)
        *(.bss .bss.*)
        . = ALIGN (4);
        _ebss = ABSOLUTE(.);
    } >RAM

    /* Heap: the rest of low SRAM, up to the ROM-loadable region. */
    . = ALIGN(4);
    _heap_start = ABSOLUTE(.);
    _heap_end = ORIGIN(RAM) + LENGTH(RAM);

    /* Mutable global variables. This data (in the SRAM segment) is
     * initialized by the ROM bootloader.
     */
    .data : ALIGN(4)
    {
        . = ALIGN (4);
        _sdata = ABSOLUTE(.);
        *(.sdata)
        *(.data .data.*)
        *(.dram*)
        . = ALIGN (4);
        _edata = ABSOLUTE(.);
    } >LOADRAM

    /* All code. Loaded into SRAM by the ROM bootloader.
     * The vector table must be 256-byte aligned (the CLIC mtvec base is
     * 64-byte aligned; 256 keeps a safety margin and matches other ESP32
     * targets).
     */
    .text : ALIGN(4)
    {
        *(.init)      /* call_start_cpu0 */
        *(.iram*)
        *(.text.handleInterruptASM)
        . = ALIGN(256);
        *(.text.exception_vectors)
        *(.text .text.*)
        . = ALIGN (4);
    } > LOADRAM

    /* Constant global variables. */
    .rodata : ALIGN(4)
    {
        *(.rodata*)
        *(.srodata*)
        . = ALIGN (4);
    } >LOADRAM

    /DISCARD/ :
    {
        *(.eh_frame)     /* we don't do exception handling in C */
    }

    /* For the garbage collector. */
    .tinygo_stacksizes (INFO) :
    {
        *(.tinygo_stacksizes)
    }
    .tinygo_arm_entries (INFO) :
    {
        *(.tinygo_arm_entries)
    }
}

/* For the GC and scheduler. */
_globals_start = _sbss;
_globals_end = _edata;

/* Default stack size. */
_stack_size = 16K;

/* ROM function addresses for ESP32-C5.
 * Source: ESP-IDF components/esp_rom/esp32c5/ld/esp32c5.rom.ld
 */
PROVIDE( ets_delay_us = 0x4000003c );
