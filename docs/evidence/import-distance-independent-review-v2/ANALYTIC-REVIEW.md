# Independent analytic review of the v2 signed-IP bound

Target: `import-distance-freeze-v2`, tree `666e201a98638e7c5759ab25ffe3aed7adbf8b4c`.

## Domain and exact source reference

Each finite binary32 component is an integer multiple of 2^-149. A widened component product is an integer multiple of 2^-298, requires at most 48 significant bits, and is exactly representable in binary64. At most 16,000 such terms have magnitudes far below binary64 overflow. All nonzero products, partial sums, local residuals and correction sums remain on that 2^-298 grid and therefore above binary64's subnormal range. These facts justify using the ordinary relative-rounding model without an underflow additive term.

The full product exponent span plus the 16,000-term accumulation requires fewer than 768 significant bits. The IP reference therefore sums source products exactly before its final binary64 conversion. L1, L2 and norm component sums also fit. Cosine's product of accumulated squared norms can exceed 768 significant bits; v2 now correctly discloses that additional high-precision rounding. It is not native metric arithmetic or a database-derived reference.

## Bound derivation checked independently

Let u = 2^-53, n <= 16,000, S = sum |p_i| and gamma_n = n*u/(1-n*u).

1. The native ordinary running sum has the standard partial-sum magnitude bound |s_i| <= S_i/(1-n*u). Each conditional Neumaier expression is FastTwoSum with the larger-magnitude argument first, so its local residual e_i is exact. The local rounding error satisfies |e_i| <= u|s_(i-1)+p_i|. Bounding each partial magnitude by the final S/(1-n*u) and summing at most n terms gives sum |e_i| <= gamma_n*S.
2. The correction is an ordinary floating sum of those residuals, not a recursively compensated sum. Its error delta is bounded by gamma_n*sum |e_i| <= gamma_n^2*S. This is the term missing from v1.
3. The exact identity is s_n + sum e_i = P, where P is the exact source dot product. Final native rounding gives |native-P| <= u|P| + (1+u)|delta|. The binary64 reference R contributes at most another u|P|. Because |P| <= |R|/(1-u), the comparison error is at most 2*u/(1-u)*|R| + (1+u)*gamma_n^2*S.
4. Since 2*u/(1-u) < 4*u, the implemented analytic formula 4*u*|R| + (1+u)*gamma_n^2*S is conservative. Negating the inner product does not change its absolute error.

## Outward evaluation audit

- n*u is exactly representable for the supported integer dimensions.
- The denominator 1-n*u is moved one representable value downward before division; the positive quotient is moved upward.
- Each nonzero exact positive product is added to an upper bound on S, then moved upward. Zero terms do not create an artificial nonzero scale.
- Gamma squaring, scale multiplication, the 1+u coefficient, final/reference-rounding term and final addition all move upward. In particular, 1+u rounds to 1 in binary64, but Nextafter(1,+Inf) yields 1+2u and safely covers the exact coefficient.
- All intermediate quantities are positive and finite in this domain. No negative-factor monotonicity reversal or overflow/underflow invalidates the direction.
- All-zero products retain exactly zero bound; otherwise even an exact-zero reference gets a valid positive correction allowance.

The new owner test compares the actual bound with an exact rational evaluation of the formula. Independent additional tests also examine every local residual and the two analytic inequalities on 13 signed, scaled, reordered and full-exponent-span fixtures. They compare source sums and the final reported error using exact rational arithmetic, rather than relying on a rounded absolute-error comparison alone.

## Other metrics and ranking

The unchanged L1/L2/norm allowance is relative and has no absolute floor that could hide a zero replacing a subnormal-size result. Their nonnegative accumulation, widened differences/products and final square root admit errors far below the conservative 64*2^-52 coefficient over n <= 16,000. Cosine's normalized absolute error is bounded using the nonnegative norm sums and |dot| <= sqrt(normA^2*normB^2), avoiding a relative-only assertion near zero.

Severe signed cancellation can still produce large uncertainty relative to the true tiny result. The report retains each absolute bound and explicitly labels overlapping Top-K intervals as ambiguous; it does not certify unique mathematical neighbors there. The unchanged native signed-key tie order and separate Top-K/prefix checks still apply. Rounded interval endpoints cannot turn a true overlap into a strictly clear boundary because round-to-nearest is monotonic; they may conservatively leave a very narrow separation ambiguous.

This is a scoped derivation and regression review, not a formal proof assistant certificate or a blanket engine/release certification.
