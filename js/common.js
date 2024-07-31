// Read JSONL from stdin.
export function readInput(handle) {
	const buffSize = 1024;
	let currentLine = [];
	const buffer = new Uint8Array(buffSize);

	// Read all the available bytes
	while (true) {
		// Stdin file descriptor
		const fd = 0;
		let bytesRead = 0;
		try {
			bytesRead = Javy.IO.readSync(fd, buffer);
		} catch (e) {
			// IO.readSync fails with os error 29 when stdin closes.
			if (e.message.includes('os error 29')) {
				break;
			}
			throw new Error('Error reading from stdin');
		}

		if (bytesRead < 0) {
			throw new Error('Error reading from stdin');
			break;
		}

		if (bytesRead === 0) {
			break;
		}

		currentLine = [...currentLine, ...buffer.subarray(0, bytesRead)];

		// Split array into chunks by newline.
		let i = 0;
		for (let j = 0; i < currentLine.length; i++) {
			if (currentLine[i] === 10) {
				const chunk = currentLine.splice(j, i + 1);
				const arr = new Uint8Array(chunk);
				const json = JSON.parse(new TextDecoder().decode(arr));
				handle(json);
				j = i + 1;
			}
		}
		// Remove processed data.
		currentLine = currentLine.slice(i);
	}
}

// Write JSONL to stdout
export function writeOutput(output) {
	const encodedOutput = new TextEncoder().encode(JSON.stringify(output) + '\n');
	const buffer = new Uint8Array(encodedOutput);
	// Stdout file descriptor
	const fd = 1;
	Javy.IO.writeSync(fd, buffer);
}
