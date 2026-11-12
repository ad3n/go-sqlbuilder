//go:build !purego && !race

#include "textflag.h"

TEXT ·encodeHexBlocks(SB), NOSPLIT, $0-48
 MOVD dst_base+0(FP), R0
 MOVD src_base+24(FP), R1
 MOVD src_len+32(FP), R2
 MOVD $·hexDigits(SB), R3
 VLD1 (R3), [V7.B16]
 MOVD $15, R3
 VDUP R3, V6.B16
loop:
 VLD1.P 16(R1), [V0.B16]
 VUSHR $4, V0.B16, V1.B16
 VAND V6.B16, V0.B16, V0.B16
 VTBL V1.B16, [V7.B16], V1.B16
 VTBL V0.B16, [V7.B16], V0.B16
 VZIP1 V0.B16, V1.B16, V2.B16
 VZIP2 V0.B16, V1.B16, V3.B16
 VST1.P [V2.B16, V3.B16], 32(R0)
 SUBS $16, R2
 BNE loop
 RET
