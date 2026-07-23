#include <stdio.h>
#include <stdlib.h>

typedef struct {
    int val;
    int left;
    int right;
} Node;

int main(void) {
    int count = 10000;
    Node* nodes = (Node*)malloc(count * sizeof(Node));
    int seed = 123456789;

    for (int k = 0; k < count; k++) {
        seed = (int)(((unsigned int)seed * 1664525u + 1013904223u) & 0x7fffffff) % 2147483647;
        int val = seed % 1000000;
        if (val < 0) {
            val = -val;
        }

        nodes[k].val = val;
        nodes[k].left = -1;
        nodes[k].right = -1;

        if (k > 0) {
            int curr = 0;
            int placed = 0;
            while (!placed) {
                if (val < nodes[curr].val) {
                    if (nodes[curr].left == -1) {
                        nodes[curr].left = k;
                        placed = 1;
                    } else {
                        curr = nodes[curr].left;
                    }
                } else {
                    if (nodes[curr].right == -1) {
                        nodes[curr].right = k;
                        placed = 1;
                    } else {
                        curr = nodes[curr].right;
                    }
                }
            }
        }
    }

    long long checksum = 0;
    int* stack = (int*)malloc(count * sizeof(int));
    int top = -1;
    int curr = 0;

    while (curr != -1 || top >= 0) {
        while (curr != -1) {
            top++;
            stack[top] = curr;
            curr = nodes[curr].left;
        }

        curr = stack[top--];
        checksum = (checksum + nodes[curr].val) % 1000000007;
        curr = nodes[curr].right;
    }

    printf("%lld\n", checksum);

    free(nodes);
    free(stack);
    return 0;
}
