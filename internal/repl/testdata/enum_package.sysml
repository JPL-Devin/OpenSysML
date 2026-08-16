package D {
    enum def Color { red; green; blue; }

    enum def Level {
        attribute n : ScalarValues::Integer;

        low { :>> n = 1; }
        high { :>> n = 9; }
    }

    part def Car {
        attribute c : Color = Color::red;
        attribute l : Level = Level::high;
        attribute isRed = c == Color::red;
        attribute grade = l.n;
    }
}
