#include <stdio.h>
#include <stdlib.h>

int main(void) {
    int count = 10000;
    int* nums = (int*)malloc(count * sizeof(int));
    int seed = 123456789;
    for (int k = 0; k < count; k++) {
        seed = (int)(((unsigned int)seed * 1664525u + 1013904223u) & 0x7fffffff) % 2147483647;
        int val = seed % 1000000;
        if (val < 0) {
            val = -val;
        }
        nums[k] = val;
    }

    int* stack = (int*)malloc(count * 2 * sizeof(int));
    int top = -1;

    top++;
    stack[top] = 0;
    top++;
    stack[top] = count - 1;

    while (top >= 0) {
        int high = stack[top--];
        int low = stack[top--];

        int pivot = nums[high];
        int i = low - 1;
        for (int j = low; j < high; j++) {
            if (nums[j] <= pivot) {
                i++;
                int temp = nums[i];
                nums[i] = nums[j];
                nums[j] = temp;
            }
        }

        int temp = nums[i + 1];
        nums[i + 1] = nums[high];
        nums[high] = temp;
        int p = i + 1;

        if (p - 1 > low) {
            top++;
            stack[top] = low;
            top++;
            stack[top] = p - 1;
        }

        if (p + 1 < high) {
            top++;
            stack[top] = p + 1;
            top++;
            stack[top] = high;
        }
    }

    long long checksum = 0;
    for (int idx = 0; idx < count; idx++) {
        checksum = (checksum + nums[idx]) % 1000000007;
    }

    printf("%lld\n", checksum);

    free(nums);
    free(stack);
    return 0;
}
