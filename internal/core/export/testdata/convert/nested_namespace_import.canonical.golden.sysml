package NestedNamespaceImport {
    package Pkg1 {
        part def A;
        package Pkg11 {
            part def B;
        }
    }
    package Pkg2 {
        private import Pkg1::*;
        public import Pkg1::**;
        private import Pkg1::*::**;
        part a : Pkg1::A;
        part b : Pkg1::Pkg11::B;
    }
}
