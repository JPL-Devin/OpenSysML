package Relations {
    part def Base;
    part def Sub specializes Base;
    part def Other {
        part b : Base;
        part r redefines b;
        part s subsets b;
    }
}
