package UserKeywords {
	#scenario def DeviceFailure {
		#cause 'battery old' {
			:>> probability = 0.01;
		}

		#failure 'device shutoff';
	}
}
