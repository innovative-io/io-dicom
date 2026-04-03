#!/bin/bash
# generate-sample-dicoms.sh
#
# Generates synthetic DICOM files for testing and demonstration purposes.
# Uses the io-dicom library (Go) to generate valid DICOM objects.
#
# Usage: ./scripts/generate-sample-dicoms.sh
# 

SAMPLES_DIR="samples"
GENERATOR_BIN="cmd/generate-sample-dicoms/main"

echo "Generating synthetic DICOM files using io-dicom library..."
echo "Output directory: $SAMPLES_DIR"
echo ""

# Create samples directory if needed
mkdir -p "$SAMPLES_DIR"

# Check if the generator binary is available, otherwise compile it
if [ ! -f "$GENERATOR_BIN" ]; then
    echo "Building generator binary..."
    cd cmd/generate-sample-dicoms && go build -o main . && cd ../..
    if [ $? -ne 0 ]; then
        echo "Error: Failed to compile generator. Ensure you have Go 1.26+ installed."
        exit 1
    fi
fi

# Run the generator
./$GENERATOR_BIN -output "$SAMPLES_DIR"
echo ""
echo "✓ Synthetic DICOM generation complete!"
echo ""
echo "Generated files:"
ls -lh "$SAMPLES_DIR"/synthetic-*.dcm 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}' || echo "  (none)"
