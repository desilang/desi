# Slicing

Desi supports Python-style slicing for lists and strings.

## Basic Slicing

```desi
let nums = [10, 20, 30, 40, 50]

# Get elements from index 1 to 3 (exclusive)
let slice1 = nums[1:4]  # [20, 30, 40]

# From beginning to index 2
let slice2 = nums[:3]   # [10, 20, 30]

# From index 2 to end
let slice3 = nums[2:]   # [30, 40, 50]
```

## Negative Indexing

Negative indices count from the end:

```desi
let nums = [10, 20, 30, 40, 50]

let last = nums[-1]     # 50 (last element)
let second_last = nums[-2]  # 40

# Last three elements
let last_three = nums[-3:]  # [30, 40, 50]
```

## Step Slicing

Use a third parameter to specify the step:

```desi
let nums = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

# Every second element
let evens = nums[::2]   # [1, 3, 5, 7, 9]

# Reverse the list
let reversed = nums[::-1]  # [10, 9, 8, 7, 6, 5, 4, 3, 2, 1]

# Every second element in reverse
let rev_evens = nums[::-2]  # [10, 8, 6, 4, 2]

# Range with step
let ranged = nums[1:7:2]    # [2, 4, 6]
```

## String Slicing

Slicing works the same way for strings:

```desi
let s = "Hello, World!"

let hello = s[:5]       # "Hello"
let world = s[7:12]     # "World"
let reversed = s[::-1]  # "!dlroW ,olleH"
```

## Custom Indexing

Classes can implement `__getitem__` to support indexing:

```desi
class MyList:
    pub data: list[int]
    
    pub def __new__(self, values: list[int]):
        self.data = values
    
    pub def __getitem__(self, index: int) -> int:
        return self.data[index]

let arr = MyList([10, 20, 30])
print(arr[0])   # 10
print(arr[-1])  # 30 (negative indexing works!)
```

!!! note
    Custom classes support negative indexing through `__getitem__` - just pass the index to the underlying list.
