// Math module C runtime - wraps libc math.h functions
// All functions prefixed with __math_ for namespacing.

#include <math.h>
#include <stdlib.h>
#include <time.h>
#include <float.h>

// ============================================================
// Constants (exposed as functions for safe extern binding)
// ============================================================

double __math_pi(void)  { return M_PI; }
double __math_e(void)   { return M_E; }
double __math_tau(void) { return 2.0 * M_PI; }
double __math_inf(void) { return INFINITY; }
double __math_nan(void) { return NAN; }

// ============================================================
// Trigonometric Functions
// ============================================================

double __math_sin(double x)            { return sin(x); }
double __math_cos(double x)            { return cos(x); }
double __math_tan(double x)            { return tan(x); }
double __math_asin(double x)           { return asin(x); }
double __math_acos(double x)           { return acos(x); }
double __math_atan(double x)           { return atan(x); }
double __math_atan2(double y, double x){ return atan2(y, x); }
double __math_sinh(double x)           { return sinh(x); }
double __math_cosh(double x)           { return cosh(x); }
double __math_tanh(double x)           { return tanh(x); }

// ============================================================
// Power & Logarithmic Functions
// ============================================================

double __math_sqrt(double x)             { return sqrt(x); }
double __math_cbrt(double x)             { return cbrt(x); }
double __math_pow(double x, double y)    { return pow(x, y); }
double __math_exp(double x)              { return exp(x); }
double __math_log(double x)              { return log(x); }
double __math_log2(double x)             { return log2(x); }
double __math_log10(double x)            { return log10(x); }
double __math_hypot(double x, double y)  { return hypot(x, y); }

// ============================================================
// Rounding Functions
// ============================================================

double __math_floor(double x) { return floor(x); }
double __math_ceil(double x)  { return ceil(x); }
// Banker's rounding (IEEE 754, Python-style): half to even
// round(2.5) -> 2.0, round(3.5) -> 4.0, round(-2.5) -> -2.0
double __math_round(double x) {
    double r = round(x);
    // Check if exactly at .5 boundary
    double diff = x - floor(x);
    if (fabs(diff - 0.5) < 1e-15) {
        // Round to even
        double down = floor(x);
        double up = ceil(x);
        if (fmod(fabs(down), 2.0) < 1e-15) return down;  // down is even
        return up;
    }
    return r;
}

// C-style rounding: half away from zero
// round_away(2.5) -> 3.0, round_away(-2.5) -> -3.0
double __math_round_away(double x) { return round(x); }

// Round to N decimal places with banker's rounding (matches Python)
// round_to(3.14159, 2) -> 3.14
// round_to(2.675, 2)   -> 2.67  (matches Python, due to float representation)
// round_to(1234.5, -2) -> 1200.0 (negative places, like Python)
double __math_round_to(double x, int places) {
    double multiplier = pow(10.0, (double)places);
    return __math_round(x * multiplier) / multiplier;
}
double __math_trunc(double x) { return trunc(x); }
double __math_fmod(double x, double y) { return fmod(x, y); }

// ============================================================
// Utility Functions
// ============================================================

double __math_abs(double x) { return fabs(x); }
int    __math_abs_int(int x) { return x < 0 ? -x : x; }

double __math_fmin(double x, double y) { return fmin(x, y); }
double __math_fmax(double x, double y) { return fmax(x, y); }

double __math_clamp(double x, double lo, double hi) {
    if (x < lo) return lo;
    if (x > hi) return hi;
    return x;
}

int __math_clamp_int(int x, int lo, int hi) {
    if (x < lo) return lo;
    if (x > hi) return hi;
    return x;
}

// Sign function: returns -1.0, 0.0, or 1.0
double __math_sign(double x) {
    if (x > 0.0) return 1.0;
    if (x < 0.0) return -1.0;
    return 0.0;
}

// ============================================================
// Random Number Generation
// ============================================================

static int __math_seeded = 0;

static void __math_ensure_seeded(void) {
    if (!__math_seeded) {
        srand((unsigned int)time(NULL));
        __math_seeded = 1;
    }
}

// Returns random float in [0.0, 1.0)
double __math_random(void) {
    __math_ensure_seeded();
    return (double)rand() / ((double)RAND_MAX + 1.0);
}

// Returns random int in [lo, hi] (inclusive)
int __math_randint(int lo, int hi) {
    __math_ensure_seeded();
    if (lo > hi) { int t = lo; lo = hi; hi = t; }
    return lo + (rand() % (hi - lo + 1));
}

// Seed the RNG
void __math_seed(int n) {
    srand((unsigned int)n);
    __math_seeded = 1;
}

// ============================================================
// Conversion Helpers
// ============================================================

// Degrees <-> Radians
double __math_radians(double degrees) { return degrees * M_PI / 180.0; }
double __math_degrees(double radians) { return radians * 180.0 / M_PI; }

// ============================================================
// Unique Functions — Not commonly found in standard math libs
// ============================================================

// Linear interpolation: lerp(a=0, b=10, t=0.5) -> 5.0
// Available in C++20, Rust, GLSL; missing from Python, Java, Go
double __math_lerp(double a, double b, double t) {
    return a + t * (b - a);
}

// Inverse lerp: given a value in [a, b], returns the t ∈ [0, 1]
// Rarely available in any language stdlib
double __math_inverse_lerp(double a, double b, double x) {
    if (a == b) return 0.0;
    return (x - a) / (b - a);
}

// Remap a value from one range to another
// map_range(50, 0, 100, 0, 1) -> 0.5
// Found in Arduino/Processing, rarely in general-purpose languages
double __math_map_range(double x, double in_lo, double in_hi, double out_lo, double out_hi) {
    if (in_lo == in_hi) return out_lo;
    return out_lo + (x - in_lo) * (out_hi - out_lo) / (in_hi - in_lo);
}

// Approximate float equality with absolute tolerance
// approx_eq(0.1 + 0.2, 0.3, 1e-9) -> true
// Rust has it in approx crate, Python has math.isclose
int __math_approx_eq(double a, double b, double epsilon) {
    return fabs(a - b) <= epsilon;
}

// Python-style is_close with relative AND absolute tolerance
// is_close(1000.0, 1000.001, 1e-6, 1e-9)
double __math_is_close(double a, double b, double rel_tol, double abs_tol) {
    double diff = fabs(a - b);
    return diff <= fmax(rel_tol * fmax(fabs(a), fabs(b)), abs_tol);
}

// Smooth Hermite interpolation (0→1 for x ∈ [edge0, edge1])
// GLSL built-in, not available in any general-purpose language stdlib
double __math_smoothstep(double edge0, double edge1, double x) {
    double t = (x - edge0) / (edge1 - edge0);
    if (t < 0.0) t = 0.0;
    if (t > 1.0) t = 1.0;
    return t * t * (3.0 - 2.0 * t);
}

// Wrap value to [lo, hi) range (modular arithmetic for any range)
// wrap(370, 0, 360) -> 10.0
// Not available in any major language stdlib
double __math_wrap(double x, double lo, double hi) {
    double range = hi - lo;
    if (range <= 0.0) return lo;
    double result = fmod(x - lo, range);
    if (result < 0.0) result += range;
    return result + lo;
}

int __math_wrap_int(int x, int lo, int hi) {
    int range = hi - lo;
    if (range <= 0) return lo;
    int result = (x - lo) % range;
    if (result < 0) result += range;
    return result + lo;
}

// Greatest common divisor (Euclidean algorithm)
// Python math.gcd, Rust num::gcd, missing from C/C++/Java stdlib
int __math_gcd(int a, int b) {
    if (a < 0) a = -a;
    if (b < 0) b = -b;
    while (b != 0) {
        int t = b;
        b = a % b;
        a = t;
    }
    return a;
}

// Least common multiple
int __math_lcm(int a, int b) {
    if (a == 0 || b == 0) return 0;
    int g = __math_gcd(a, b);
    // Use abs to handle negatives
    int aa = a < 0 ? -a : a;
    int bb = b < 0 ? -b : b;
    return (aa / g) * bb;
}

// Factorial (iterative, up to 20! fits in int64)
long long __math_factorial(int n) {
    if (n < 0) return -1; // error sentinel
    if (n <= 1) return 1;
    long long result = 1;
    for (int i = 2; i <= n; i++) {
        result *= i;
    }
    return result;
}

// Fibonacci (iterative, O(n))
long long __math_fib(int n) {
    if (n < 0) return -1;
    if (n <= 1) return n;
    long long a = 0, b = 1;
    for (int i = 2; i <= n; i++) {
        long long t = a + b;
        a = b;
        b = t;
    }
    return b;
}

// Step function: returns 0.0 if x < edge, 1.0 otherwise
// GLSL built-in, not in any general-purpose language
double __math_step(double edge, double x) {
    return x < edge ? 0.0 : 1.0;
}
