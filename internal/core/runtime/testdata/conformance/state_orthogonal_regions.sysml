package Test {
	state def TrafficLight {
		// Pedestrian region - controls walk/don't walk signals
		region pedestrian {
			initial start;
			state Walk;
			state DontWalk;
			
			succession first start then Walk;
			transition Walk to DontWalk when pedestrianTimer;
			transition DontWalk to Walk when pedestrianTimer;
		}
		
		// Vehicle region - controls traffic light colors
		region vehicle {
			initial start;
			state Green;
			state Yellow;
			state Red;
			
			succession first start then Green;
			transition Green to Yellow when vehicleTimer;
			transition Yellow to Red when vehicleTimer;
			transition Red to Green when vehicleTimer;
		}
	}
}
