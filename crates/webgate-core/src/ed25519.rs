#![forbid(unsafe_code)]

/// Pure safe Rust SHA-512 implementation (FIPS 180-4).
#[derive(Clone)]
pub struct Sha512 {
    state: [u64; 8],
    buffer: [u8; 128],
    buf_len: usize,
    total_len: u128,
}

const SHA512_K: [u64; 80] = [
    0x428a2f98d728ae22,
    0x7137449123ef65cd,
    0xb5c0fbcfec4d3b2f,
    0xe9b5dba58189dbbc,
    0x3956c25bf348b538,
    0x59f111f1b605d019,
    0x923f82a4af194f9b,
    0xab1c5ed5da6d8118,
    0xd807aa98a3030242,
    0x12835b0145706fbe,
    0x243185be4ee4b28c,
    0x550c7dc3d5ffb4e2,
    0x72be5d74f27b896f,
    0x80deb1fe3b1696b1,
    0x9bdc06a725c71235,
    0xc19bf174cf692694,
    0xe49b69c19ef14ad2,
    0xefbe4786384f25e3,
    0x0fc19dc68b8cd5b5,
    0x240ca1cc77ac9c65,
    0x2de92c6f592b0275,
    0x4a7484aa6ea6e483,
    0x5cb0a9dcbd41fbd4,
    0x76f988da831153b5,
    0x983e5152ee66dfab,
    0xa831c66d2db43210,
    0xb00327c898fb213f,
    0xbf597fc7beef0ee4,
    0xc6e00bf33da88fc2,
    0xd5a79147930aa725,
    0x06ca6351e003826f,
    0x142929670a0e6e70,
    0x27b70a8546d22ffc,
    0x2e1b21385c26c926,
    0x4d2c6dfc5ac42aed,
    0x53380d139d95b3df,
    0x650a73548baf63de,
    0x766a0abb3c77b2a8,
    0x81c2c92e47edaee6,
    0x92722c851482353b,
    0xa2bfe8a14cf10364,
    0xa81a664bbc423001,
    0xc24b8b70d0f89791,
    0xc76c51a30654be30,
    0xd192e819d6ef5218,
    0xd69906245565a910,
    0xf40e35855771202a,
    0x106aa07032bbd1b8,
    0x19a4c116b8d2d0c8,
    0x1e376c085141ab53,
    0x2748774cdf8eeb99,
    0x34b0bcb5e19b48a8,
    0x391c0cb3c5c95a63,
    0x4ed8aa4ae3418acb,
    0x5b9cca4f7763e373,
    0x682e6ff3d6b2b8a3,
    0x748f82ee5defb2fc,
    0x78a5636f43172f60,
    0x84c87814a1f0ab72,
    0x8cc702081a6439ec,
    0x90befffa23631e28,
    0xa4506cebde82bde9,
    0xbef9a3f7b2c67915,
    0xc67178f2e372532b,
    0xca273eceea26619c,
    0xd186b8c721c0c207,
    0xeada7dd6cde0eb1e,
    0xf57d4f7fee6ed178,
    0x06f067aa72176fba,
    0x0a637dc5a2c898a6,
    0x113f9804bef90dae,
    0x1b710b35131c471b,
    0x28db77f523047d84,
    0x32caab7b40c72493,
    0x3c9ebe0a15c9bebc,
    0x431d67c49c100d4c,
    0x4cc5d4becb3e42b6,
    0x597f299cfc657e2a,
    0x5fcb6fab3ad6faec,
    0x6c44198c4a475817,
];

impl Default for Sha512 {
    fn default() -> Self {
        Self::new()
    }
}

impl Sha512 {
    #[must_use]
    pub fn new() -> Self {
        Self {
            state: [
                0x6a09e667f3bcc908,
                0xbb67ae8584caa73b,
                0x3c6ef372fe94f82b,
                0xa54ff53a5f1d36f1,
                0x510e527fade682d1,
                0x9b05688c2b3e6c1f,
                0x1f83d9abfb41bd6b,
                0x5be0cd19137e2179,
            ],
            buffer: [0u8; 128],
            buf_len: 0,
            total_len: 0,
        }
    }

    pub fn update(&mut self, data: &[u8]) {
        let mut offset = 0;
        let mut len = data.len();
        self.total_len += len as u128;

        if self.buf_len > 0 {
            let to_fill = 128 - self.buf_len;
            if len < to_fill {
                self.buffer[self.buf_len..self.buf_len + len].copy_from_slice(data);
                self.buf_len += len;
                return;
            }
            self.buffer[self.buf_len..128].copy_from_slice(&data[..to_fill]);
            let block = self.buffer;
            self.process_block(&block);
            self.buf_len = 0;
            offset += to_fill;
            len -= to_fill;
        }

        while len >= 128 {
            let mut block = [0u8; 128];
            block.copy_from_slice(&data[offset..offset + 128]);
            self.process_block(&block);
            offset += 128;
            len -= 128;
        }

        if len > 0 {
            self.buffer[..len].copy_from_slice(&data[offset..offset + len]);
            self.buf_len = len;
        }
    }

    fn process_block(&mut self, block: &[u8; 128]) {
        let mut w = [0u64; 80];
        for (i, chunk) in block.chunks_exact(8).enumerate() {
            w[i] = u64::from_be_bytes(chunk.try_into().unwrap_or([0u8; 8]));
        }

        for i in 16..80 {
            let s0 = w[i - 15].rotate_right(1) ^ w[i - 15].rotate_right(8) ^ (w[i - 15] >> 7);
            let s1 = w[i - 2].rotate_right(19) ^ w[i - 2].rotate_right(61) ^ (w[i - 2] >> 6);
            w[i] = w[i - 16]
                .wrapping_add(s0)
                .wrapping_add(w[i - 7])
                .wrapping_add(s1);
        }

        let mut a = self.state[0];
        let mut b = self.state[1];
        let mut c = self.state[2];
        let mut d = self.state[3];
        let mut e = self.state[4];
        let mut f = self.state[5];
        let mut g = self.state[6];
        let mut h = self.state[7];

        for i in 0..80 {
            let s1 = e.rotate_right(14) ^ e.rotate_right(18) ^ e.rotate_right(41);
            let ch = (e & f) ^ ((!e) & g);
            let temp1 = h
                .wrapping_add(s1)
                .wrapping_add(ch)
                .wrapping_add(SHA512_K[i])
                .wrapping_add(w[i]);
            let s0 = a.rotate_right(28) ^ a.rotate_right(34) ^ a.rotate_right(39);
            let maj = (a & b) ^ (a & c) ^ (b & c);
            let temp2 = s0.wrapping_add(maj);

            h = g;
            g = f;
            f = e;
            e = d.wrapping_add(temp1);
            d = c;
            c = b;
            b = a;
            a = temp1.wrapping_add(temp2);
        }

        self.state[0] = self.state[0].wrapping_add(a);
        self.state[1] = self.state[1].wrapping_add(b);
        self.state[2] = self.state[2].wrapping_add(c);
        self.state[3] = self.state[3].wrapping_add(d);
        self.state[4] = self.state[4].wrapping_add(e);
        self.state[5] = self.state[5].wrapping_add(f);
        self.state[6] = self.state[6].wrapping_add(g);
        self.state[7] = self.state[7].wrapping_add(h);
    }

    #[must_use]
    pub fn finalize(mut self) -> [u8; 64] {
        let total_bits = self.total_len * 8;
        let buf_len = self.buf_len;
        if buf_len < 112 {
            self.buffer[buf_len] = 0x80;
            self.buffer[buf_len + 1..112].fill(0);
            self.buffer[112..128].copy_from_slice(&total_bits.to_be_bytes());
            let block = self.buffer;
            self.process_block(&block);
        } else {
            // Fill current block with 0x80 and zeros, process it, then second block with zeros and length
            self.buffer[buf_len] = 0x80;
            self.buffer[buf_len + 1..128].fill(0);
            let block1 = self.buffer;
            self.process_block(&block1);

            let mut block2 = [0u8; 128];
            block2[112..128].copy_from_slice(&total_bits.to_be_bytes());
            self.process_block(&block2);
        }

        let mut out = [0u8; 64];
        for (i, val) in self.state.iter().enumerate() {
            out[i * 8..(i + 1) * 8].copy_from_slice(&val.to_be_bytes());
        }
        out
    }

    #[must_use]
    pub fn digest(data: &[u8]) -> [u8; 64] {
        let mut hasher = Self::new();
        hasher.update(data);
        hasher.finalize()
    }
}

// ---------------------------------------------------------------------------
// Curve25519 & Ed25519 arithmetic (RFC 8032)
// Field element modulo p = 2^255 - 19
// Represented as 5 limbs of 51 bits each: a0 + a1*2^51 + a2*2^102 + a3*2^153 + a4*2^204
// ---------------------------------------------------------------------------

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
struct FieldElement([u64; 5]);

impl FieldElement {
    const ZERO: Self = Self([0, 0, 0, 0, 0]);
    const ONE: Self = Self([1, 0, 0, 0, 0]);

    // d = -121665/121666 mod 2^255-19
    const D: Self = Self([
        0x00034dca135978a3,
        0x0001a8283b156ebd,
        0x0005e7a26001c029,
        0x000739c663a03cbb,
        0x00052036cee2b6ff,
    ]);

    // 2*d mod 2^255-19
    const TWO_D: Self = Self([
        0x00069b9426b2f159,
        0x00035050762add7a,
        0x0003cf44c0038052,
        0x0006738cc7407977,
        0x0002406d9dc56dff,
    ]);

    // I = sqrt(-1) mod 2^255-19
    const SQRT_M1: Self = Self([
        0x00061b274a0ea0b0,
        0x0000d5a5fc8f189d,
        0x0007ef5e9cbd0c60,
        0x00078595a6804c9e,
        0x0002b8324804fc1d,
    ]);

    #[must_use]
    fn from_bytes(bytes: &[u8; 32]) -> Self {
        let mut limbs = [0u64; 5];
        let mut r = [0u8; 32];
        r.copy_from_slice(bytes);
        r[31] &= 0x7f; // clear sign bit

        let load8 =
            |i: usize| -> u64 { u64::from_le_bytes(r[i..i + 8].try_into().unwrap_or([0u8; 8])) };

        limbs[0] = load8(0) & 0x7ffffffffffff;
        limbs[1] = (load8(6) >> 3) & 0x7ffffffffffff;
        limbs[2] = (load8(12) >> 6) & 0x7ffffffffffff;
        limbs[3] = (load8(19) >> 1) & 0x7ffffffffffff;
        limbs[4] = (load8(24) >> 12) & 0x7ffffffffffff;

        Self(limbs)
    }

    #[must_use]
    fn to_bytes(mut self) -> [u8; 32] {
        self.carry();
        self.carry();
        self.carry();

        let mut q = (self.0[0] + 19) >> 51;
        q = (self.0[1] + q) >> 51;
        q = (self.0[2] + q) >> 51;
        q = (self.0[3] + q) >> 51;
        q = (self.0[4] + q) >> 51;

        self.0[0] += 19 * q;
        self.carry();

        let mut bytes = [0u8; 32];
        let h0 = self.0[0];
        let h1 = self.0[1];
        let h2 = self.0[2];
        let h3 = self.0[3];
        let h4 = self.0[4];

        bytes[0] = h0 as u8;
        bytes[1] = (h0 >> 8) as u8;
        bytes[2] = (h0 >> 16) as u8;
        bytes[3] = (h0 >> 24) as u8;
        bytes[4] = (h0 >> 32) as u8;
        bytes[5] = (h0 >> 40) as u8;
        bytes[6] = ((h0 >> 48) | (h1 << 3)) as u8;
        bytes[7] = (h1 >> 5) as u8;
        bytes[8] = (h1 >> 13) as u8;
        bytes[9] = (h1 >> 21) as u8;
        bytes[10] = (h1 >> 29) as u8;
        bytes[11] = (h1 >> 37) as u8;
        bytes[12] = ((h1 >> 45) | (h2 << 6)) as u8;
        bytes[13] = (h2 >> 2) as u8;
        bytes[14] = (h2 >> 10) as u8;
        bytes[15] = (h2 >> 18) as u8;
        bytes[16] = (h2 >> 26) as u8;
        bytes[17] = (h2 >> 34) as u8;
        bytes[18] = (h2 >> 42) as u8;
        bytes[19] = ((h2 >> 50) | (h3 << 1)) as u8;
        bytes[20] = (h3 >> 7) as u8;
        bytes[21] = (h3 >> 15) as u8;
        bytes[22] = (h3 >> 23) as u8;
        bytes[23] = (h3 >> 31) as u8;
        bytes[24] = (h3 >> 39) as u8;
        bytes[25] = ((h3 >> 47) | (h4 << 4)) as u8;
        bytes[26] = (h4 >> 4) as u8;
        bytes[27] = (h4 >> 12) as u8;
        bytes[28] = (h4 >> 20) as u8;
        bytes[29] = (h4 >> 28) as u8;
        bytes[30] = (h4 >> 36) as u8;
        bytes[31] = (h4 >> 44) as u8;

        bytes
    }

    fn carry(&mut self) {
        let mask = 0x7ffffffffffff;
        let c0 = self.0[0] >> 51;
        self.0[0] &= mask;
        self.0[1] += c0;

        let c1 = self.0[1] >> 51;
        self.0[1] &= mask;
        self.0[2] += c1;

        let c2 = self.0[2] >> 51;
        self.0[2] &= mask;
        self.0[3] += c2;

        let c3 = self.0[3] >> 51;
        self.0[3] &= mask;
        self.0[4] += c3;

        let c4 = self.0[4] >> 51;
        self.0[4] &= mask;
        self.0[0] += c4 * 19;
    }

    #[must_use]
    fn add(self, rhs: Self) -> Self {
        Self([
            self.0[0] + rhs.0[0],
            self.0[1] + rhs.0[1],
            self.0[2] + rhs.0[2],
            self.0[3] + rhs.0[3],
            self.0[4] + rhs.0[4],
        ])
    }

    #[must_use]
    fn sub(self, rhs: Self) -> Self {
        // 4 * p in limbs to avoid underflow
        const P0: u64 = 0x7ffffffffffed * 4;
        const P1: u64 = 0x7ffffffffffff * 4;
        let mut res = Self([
            self.0[0] + P0 - rhs.0[0],
            self.0[1] + P1 - rhs.0[1],
            self.0[2] + P1 - rhs.0[2],
            self.0[3] + P1 - rhs.0[3],
            self.0[4] + P1 - rhs.0[4],
        ]);
        res.carry();
        res
    }

    #[must_use]
    fn mul(self, rhs: Self) -> Self {
        let a = self.0;
        let b = rhs.0;

        let m = |x: u64, y: u64| -> u128 { (x as u128) * (y as u128) };
        let m19 = |x: u64, y: u64| -> u128 { (x as u128) * (y as u128) * 19 };

        let r0 =
            m(a[0], b[0]) + m19(a[1], b[4]) + m19(a[2], b[3]) + m19(a[3], b[2]) + m19(a[4], b[1]);
        let r1 =
            m(a[0], b[1]) + m(a[1], b[0]) + m19(a[2], b[4]) + m19(a[3], b[3]) + m19(a[4], b[2]);
        let r2 = m(a[0], b[2]) + m(a[1], b[1]) + m(a[2], b[0]) + m19(a[3], b[4]) + m19(a[4], b[3]);
        let r3 = m(a[0], b[3]) + m(a[1], b[2]) + m(a[2], b[1]) + m(a[3], b[0]) + m19(a[4], b[4]);
        let r4 = m(a[0], b[4]) + m(a[1], b[3]) + m(a[2], b[2]) + m(a[3], b[1]) + m(a[4], b[0]);

        let mask = 0x7ffffffffffff;
        let mut out = [0u64; 5];

        let c0 = (r0 >> 51) as u64;
        out[0] = (r0 as u64) & mask;

        let r1 = r1 + (c0 as u128);
        let c1 = (r1 >> 51) as u64;
        out[1] = (r1 as u64) & mask;

        let r2 = r2 + (c1 as u128);
        let c2 = (r2 >> 51) as u64;
        out[2] = (r2 as u64) & mask;

        let r3 = r3 + (c2 as u128);
        let c3 = (r3 >> 51) as u64;
        out[3] = (r3 as u64) & mask;

        let r4 = r4 + (c3 as u128);
        let c4 = (r4 >> 51) as u64;
        out[4] = (r4 as u64) & mask;

        out[0] += c4 * 19;
        let mut res = Self(out);
        res.carry();
        res
    }

    #[must_use]
    fn square(self) -> Self {
        self.mul(self)
    }

    #[must_use]
    fn pow22523(self) -> Self {
        // x^(2^252 - 3)
        let x = self.square(); // 2
        let t2 = x.mul(self); // 3
        let mut t = t2.square(); // 6
        t = t.square(); // 12
        let t4 = t.mul(t2); // 15
        t = t4.square(); // 30
        let t5 = t.mul(self); // 31
        t = t5;
        for _ in 0..5 {
            t = t.square();
        }
        let t10 = t.mul(t5); // 2^10 - 1
        t = t10;
        for _ in 0..10 {
            t = t.square();
        }
        let t20 = t.mul(t10); // 2^20 - 1
        t = t20;
        for _ in 0..20 {
            t = t.square();
        }
        let t40 = t.mul(t20); // 2^40 - 1
        t = t40;
        for _ in 0..10 {
            t = t.square();
        }
        let t50 = t.mul(t10); // 2^50 - 1
        t = t50;
        for _ in 0..50 {
            t = t.square();
        }
        let t100 = t.mul(t50); // 2^100 - 1
        t = t100;
        for _ in 0..100 {
            t = t.square();
        }
        let t200 = t.mul(t100); // 2^200 - 1
        t = t200;
        for _ in 0..50 {
            t = t.square();
        }
        let t250 = t.mul(t50); // 2^250 - 1
        t = t250.square().square(); // 2^252 - 4
        t.mul(self) // 2^252 - 3
    }

    #[must_use]
    fn invert(self) -> Self {
        // a^(p-2) where p-2 = 2^255 - 21
        let p22523 = self.pow22523();
        let t = p22523.square().square().square();
        t.mul(self.square().mul(self))
    }

    #[must_use]
    fn is_negative(self) -> bool {
        (self.to_bytes()[0] & 1) != 0
    }

    #[must_use]
    fn is_zero(self) -> bool {
        self.to_bytes() == [0u8; 32]
    }
}

// ---------------------------------------------------------------------------
// Extended Edwards Point on Ed25519: (X, Y, Z, T) with x = X/Z, y = Y/Z, xy = T/Z
// ---------------------------------------------------------------------------

#[derive(Clone, Copy, Debug)]
struct EdwardsPoint {
    x: FieldElement,
    y: FieldElement,
    z: FieldElement,
    t: FieldElement,
}

impl EdwardsPoint {
    const IDENTITY: Self = Self {
        x: FieldElement::ZERO,
        y: FieldElement::ONE,
        z: FieldElement::ONE,
        t: FieldElement::ZERO,
    };

    // Base point B = (x, 4/5)
    #[must_use]
    fn base() -> Self {
        let y = FieldElement([
            0x0006666666666658,
            0x0004cccccccccccc,
            0x0001999999999999,
            0x0003333333333333,
            0x0006666666666666,
        ]);
        let x = FieldElement([
            0x00062d608f25d51a,
            0x000412a4b4f6592a,
            0x00075b7171a4b31d,
            0x0001ff60527118fe,
            0x000216936d3cd6e5,
        ]);
        Self {
            x,
            y,
            z: FieldElement::ONE,
            t: x.mul(y),
        }
    }

    #[must_use]
    fn add(self, rhs: Self) -> Self {
        let a = self.y.sub(self.x).mul(rhs.y.sub(rhs.x));
        let b = self.y.add(self.x).mul(rhs.y.add(rhs.x));
        let c = self.t.mul(FieldElement::TWO_D).mul(rhs.t);
        let d = self.z.mul(FieldElement([2, 0, 0, 0, 0])).mul(rhs.z);
        let e = b.sub(a);
        let f = d.sub(c);
        let g = d.add(c);
        let h = b.add(a);

        Self {
            x: e.mul(f),
            y: g.mul(h),
            z: f.mul(g),
            t: e.mul(h),
        }
    }

    #[must_use]
    fn double(self) -> Self {
        let a = self.x.square();
        let b = self.y.square();
        let c = self.z.square().mul(FieldElement([2, 0, 0, 0, 0]));
        let d = a.add(b);
        let e = self.x.add(self.y).square().sub(d);
        let g = b.sub(a);
        let f = c.sub(g);
        let h = d;

        Self {
            x: e.mul(f),
            y: g.mul(h),
            z: f.mul(g),
            t: e.mul(h),
        }
    }

    #[must_use]
    fn compress(self) -> [u8; 32] {
        let z_inv = self.z.invert();
        let x = self.x.mul(z_inv);
        let y = self.y.mul(z_inv);
        let mut s = y.to_bytes();
        if x.is_negative() {
            s[31] |= 0x80;
        }
        s
    }

    #[must_use]
    fn decompress(bytes: &[u8; 32]) -> Option<Self> {
        let sign_bit = (bytes[31] & 0x80) != 0;
        let y = FieldElement::from_bytes(bytes);

        // x^2 = (y^2 - 1) / (d*y^2 + 1)
        let u = y.square().sub(FieldElement::ONE);
        let v = FieldElement::D.mul(y.square()).add(FieldElement::ONE);
        let v_inv = v.invert();
        let x2 = u.mul(v_inv);

        if x2.is_zero() {
            if sign_bit {
                return None;
            }
            return Some(Self {
                x: FieldElement::ZERO,
                y,
                z: FieldElement::ONE,
                t: FieldElement::ZERO,
            });
        }

        // Compute candidate square root: x = (u * v^3) * (u * v^7)^((p-5)/8)
        let uv3 = u.mul(v.square().mul(v));
        let uv7 = uv3.mul(v.square().mul(v.square()));
        let mut x = uv3.mul(uv7.pow22523());

        // Check if x^2 == x2
        if x.square().to_bytes() != x2.to_bytes() {
            x = x.mul(FieldElement::SQRT_M1);
            if x.square().to_bytes() != x2.to_bytes() {
                return None;
            }
        }

        if x.is_negative() != sign_bit {
            x = FieldElement::ZERO.sub(x);
        }

        Some(Self {
            x,
            y,
            z: FieldElement::ONE,
            t: x.mul(y),
        })
    }

    #[must_use]
    fn scalar_mul(self, scalar: &[u8; 32]) -> Self {
        let mut result = Self::IDENTITY;
        for i in (0..256).rev() {
            result = result.double();
            let byte_idx = i / 8;
            let bit_idx = i % 8;
            if ((scalar[byte_idx] >> bit_idx) & 1) != 0 {
                result = result.add(self);
            }
        }
        result
    }
}

// ---------------------------------------------------------------------------
// Scalar reduction modulo L = 2^252 + 27742317777372353535851937790883648493
// ---------------------------------------------------------------------------

const ORDER_L: [u64; 4] = [
    0x5812631a5cf5d3ed,
    0x14def9dea2f79cd6,
    0x0000000000000000,
    0x1000000000000000,
];

#[must_use]
fn scalar_reduce_64(input: &[u8; 64]) -> [u8; 32] {
    let mut num = [0u64; 8];
    for (i, chunk) in input.chunks_exact(8).enumerate() {
        num[i] = u64::from_le_bytes(chunk.try_into().unwrap_or([0u8; 8]));
    }

    let mut rem = [0u64; 4];

    for bit in (0..512).rev() {
        let word_idx = bit / 64;
        let bit_idx = bit % 64;
        let next_bit = (num[word_idx] >> bit_idx) & 1;

        let mut carry = next_bit;
        for item in &mut rem {
            let val = (*item << 1) | carry;
            carry = *item >> 63;
            *item = val;
        }

        if carry > 0 || scalar_gte(&rem, &ORDER_L) {
            let mut borrow = 0u64;
            for (j, item) in rem.iter_mut().enumerate() {
                let (diff, b1) = item.overflowing_sub(ORDER_L[j]);
                let (diff2, b2) = diff.overflowing_sub(borrow);
                *item = diff2;
                borrow = (b1 as u64) + (b2 as u64);
            }
        }
    }

    let mut out = [0u8; 32];
    for (i, val) in rem.iter().enumerate() {
        out[i * 8..(i + 1) * 8].copy_from_slice(&val.to_le_bytes());
    }
    out
}

#[must_use]
fn scalar_gte(a: &[u64; 4], b: &[u64; 4]) -> bool {
    for i in (0..4).rev() {
        if a[i] > b[i] {
            return true;
        }
        if a[i] < b[i] {
            return false;
        }
    }
    true
}

#[must_use]
fn scalar_mul_add(a: &[u8; 32], b: &[u8; 32], c: &[u8; 32]) -> [u8; 32] {
    let mut a_limbs = [0u64; 8];
    let mut b_limbs = [0u64; 8];
    let mut c_limbs = [0u64; 8];

    for i in 0..8 {
        a_limbs[i] =
            u32::from_le_bytes(a[i * 4..(i + 1) * 4].try_into().unwrap_or([0u8; 4])) as u64;
        b_limbs[i] =
            u32::from_le_bytes(b[i * 4..(i + 1) * 4].try_into().unwrap_or([0u8; 4])) as u64;
        c_limbs[i] =
            u32::from_le_bytes(c[i * 4..(i + 1) * 4].try_into().unwrap_or([0u8; 4])) as u64;
    }

    let mut prod = [0u128; 16];
    for i in 0..8 {
        for j in 0..8 {
            prod[i + j] += (a_limbs[i] as u128) * (b_limbs[j] as u128);
        }
        prod[i] += c_limbs[i] as u128;
    }

    let mut carry = 0u128;
    let mut prod32 = [0u32; 16];
    for i in 0..16 {
        let total = prod[i] + carry;
        prod32[i] = total as u32;
        carry = total >> 32;
    }

    let mut bytes512 = [0u8; 64];
    for (i, val) in prod32.iter().enumerate() {
        bytes512[i * 4..(i + 1) * 4].copy_from_slice(&val.to_le_bytes());
    }

    scalar_reduce_64(&bytes512)
}

// ---------------------------------------------------------------------------
// Public RFC 8032 Ed25519 API
// ---------------------------------------------------------------------------

/// Ed25519 Keypair (RFC 8032).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Ed25519Keypair {
    pub seed: [u8; 32],
    pub public_key: [u8; 32],
}

impl Ed25519Keypair {
    #[must_use]
    pub fn from_seed(seed: [u8; 32]) -> Self {
        let h = Sha512::digest(&seed);
        let mut a = [0u8; 32];
        a.copy_from_slice(&h[..32]);
        a[0] &= 248;
        a[31] &= 127;
        a[31] |= 64;

        let point_a = EdwardsPoint::base().scalar_mul(&a);
        let public_key = point_a.compress();

        Self { seed, public_key }
    }

    #[must_use]
    pub fn sign(&self, message: &[u8]) -> [u8; 64] {
        let h = Sha512::digest(&self.seed);
        let mut a = [0u8; 32];
        a.copy_from_slice(&h[..32]);
        a[0] &= 248;
        a[31] &= 127;
        a[31] |= 64;

        let mut prefix_hasher = Sha512::new();
        prefix_hasher.update(&h[32..64]);
        prefix_hasher.update(message);
        let r_hash = prefix_hasher.finalize();
        let r_scalar = scalar_reduce_64(&r_hash);

        let point_r = EdwardsPoint::base().scalar_mul(&r_scalar);
        let r_bytes = point_r.compress();

        let mut k_hasher = Sha512::new();
        k_hasher.update(&r_bytes);
        k_hasher.update(&self.public_key);
        k_hasher.update(message);
        let k_hash = k_hasher.finalize();
        let k_scalar = scalar_reduce_64(&k_hash);

        let s_scalar = scalar_mul_add(&k_scalar, &a, &r_scalar);

        let mut signature = [0u8; 64];
        signature[..32].copy_from_slice(&r_bytes);
        signature[32..].copy_from_slice(&s_scalar);
        signature
    }

    #[must_use]
    pub fn public_key_hex(&self) -> String {
        let mut hex = String::with_capacity(64);
        for byte in self.public_key {
            use std::fmt::Write;
            let _ = write!(hex, "{byte:02x}");
        }
        hex
    }
}

/// Verify an RFC 8032 Ed25519 signature.
#[must_use]
pub fn ed25519_verify(public_key: &[u8; 32], message: &[u8], signature: &[u8; 64]) -> bool {
    let mut r_bytes = [0u8; 32];
    let mut s_bytes = [0u8; 32];
    r_bytes.copy_from_slice(&signature[..32]);
    s_bytes.copy_from_slice(&signature[32..]);

    let mut s_limbs = [0u64; 4];
    for i in 0..4 {
        s_limbs[i] = u64::from_le_bytes(s_bytes[i * 8..(i + 1) * 8].try_into().unwrap_or([0u8; 8]));
    }
    if scalar_gte(&s_limbs, &ORDER_L) {
        return false;
    }

    let Some(point_a) = EdwardsPoint::decompress(public_key) else {
        return false;
    };
    let Some(point_r) = EdwardsPoint::decompress(&r_bytes) else {
        return false;
    };

    let mut k_hasher = Sha512::new();
    k_hasher.update(&r_bytes);
    k_hasher.update(public_key);
    k_hasher.update(message);
    let k_hash = k_hasher.finalize();
    let k_scalar = scalar_reduce_64(&k_hash);

    let sb = EdwardsPoint::base().scalar_mul(&s_bytes);
    let r_plus_ka = point_r.add(point_a.scalar_mul(&k_scalar));

    sb.compress() == r_plus_ka.compress()
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;

    #[test]
    fn sha512_standard_vectors() {
        let empty = Sha512::digest(b"");
        let expected_empty_hex = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e";
        let mut hex = String::new();
        for b in empty {
            use std::fmt::Write;
            let _ = write!(hex, "{b:02x}");
        }
        assert_eq!(hex, expected_empty_hex);

        let abc = Sha512::digest(b"abc");
        let expected_abc_hex = "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f";
        let mut abc_hex = String::new();
        for b in abc {
            use std::fmt::Write;
            let _ = write!(abc_hex, "{b:02x}");
        }
        assert_eq!(abc_hex, expected_abc_hex);
    }

    #[test]
    fn ed25519_rfc8032_vector_1() {
        // RFC 8032 section 7.1 Test 1
        let seed = [
            0x9d, 0x61, 0xb1, 0x9d, 0xef, 0xfd, 0x5a, 0x60, 0xba, 0x84, 0x4a, 0xf4, 0x92, 0xec,
            0x2c, 0xc4, 0x44, 0x49, 0xc5, 0x69, 0x7b, 0x32, 0x69, 0x19, 0x70, 0x3b, 0xac, 0x03,
            0x1c, 0xae, 0x7f, 0x60,
        ];
        let keypair = Ed25519Keypair::from_seed(seed);
        let expected_pub = "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a";
        assert_eq!(keypair.public_key_hex(), expected_pub);

        let msg = b"";
        let sig = keypair.sign(msg);
        let mut sig_hex = String::new();
        for b in sig {
            use std::fmt::Write;
            let _ = write!(sig_hex, "{b:02x}");
        }
        let expected_sig = "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b";
        assert_eq!(sig_hex, expected_sig);

        assert!(ed25519_verify(&keypair.public_key, msg, &sig));
        assert!(!ed25519_verify(&keypair.public_key, b"different", &sig));
    }

    #[test]
    fn ed25519_rfc8032_vector_2() {
        // RFC 8032 section 7.1 Test 2
        let seed = [
            0x4c, 0xcd, 0x08, 0x9b, 0x28, 0xff, 0x96, 0xda, 0x9d, 0xb6, 0xc3, 0x46, 0xec, 0x11,
            0x4e, 0x0f, 0x5b, 0x8a, 0x31, 0x9f, 0x35, 0xab, 0xa6, 0x24, 0xda, 0x8c, 0xf6, 0xed,
            0x4f, 0xb8, 0xa6, 0xfb,
        ];
        let keypair = Ed25519Keypair::from_seed(seed);
        let expected_pub = "3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c";
        assert_eq!(keypair.public_key_hex(), expected_pub);

        let msg = [0x72];
        let sig = keypair.sign(&msg);
        let mut sig_hex = String::new();
        for b in sig {
            use std::fmt::Write;
            let _ = write!(sig_hex, "{b:02x}");
        }
        let expected_sig = "92a009a9f0d4cab8720e820b5f642540a2b27b5416503f8fb3762223ebdb69da085ac1e43e15996e458f3613d0f11d8c387b2eaeb4302aeeb00d291612bb0c00";
        assert_eq!(sig_hex, expected_sig);
        assert!(ed25519_verify(&keypair.public_key, &msg, &sig));
    }

    #[test]
    fn ed25519_rfc8032_vector_3() {
        // RFC 8032 section 7.1 Test 3
        let seed = [
            0xc5, 0xaa, 0x8d, 0xf4, 0x3f, 0x9f, 0x83, 0x7b, 0xed, 0xb7, 0x44, 0x2f, 0x31, 0xdc,
            0xb7, 0xb1, 0x66, 0xd3, 0x85, 0x35, 0x07, 0x6f, 0x09, 0x4b, 0x85, 0xce, 0x3a, 0x2e,
            0x0b, 0x44, 0x58, 0xf7,
        ];
        let keypair = Ed25519Keypair::from_seed(seed);
        let expected_pub = "fc51cd8e6218a1a38da47ed00230f0580816ed13ba3303ac5deb911548908025";
        assert_eq!(keypair.public_key_hex(), expected_pub);

        let msg = [0xaf, 0x82];
        let sig = keypair.sign(&msg);
        let mut sig_hex = String::new();
        for b in sig {
            use std::fmt::Write;
            let _ = write!(sig_hex, "{b:02x}");
        }
        let expected_sig = "6291d657deec24024827e69c3abe01a30ce548a284743a445e3680d7db5ac3ac18ff9b538d16f290ae67f760984dc6594a7c15e9716ed28dc027beceea1ec40a";
        assert_eq!(sig_hex, expected_sig);
        assert!(ed25519_verify(&keypair.public_key, &msg, &sig));
    }
}
