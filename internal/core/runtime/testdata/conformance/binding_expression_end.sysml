package test {
    part def Sys {
        attribute a = 5;
        attribute b;
        attribute sibling = 8;
        binding expression bind b = a + 1;
    }
}
