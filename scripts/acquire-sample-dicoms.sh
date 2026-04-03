#!/bin/bash
# acquire-sample-dicoms.sh
# 
# Downloads open-source DICOM sample files from public repositories
# These files are used for testing, validation, and demonstration purposes
# 
# Usage: ./scripts/acquire-sample-dicoms.sh
#

SAMPLES_DIR="samples"
TEMP_DIR=$(mktemp -d)

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

echo -e "${YELLOW}Acquiring DICOM sample files...${NC}"
echo "Target directory: $SAMPLES_DIR"
echo ""

# Create samples directory if it doesn't exist
mkdir -p "$SAMPLES_DIR"

# Function to download a file if it doesn't already exist
download_file() {
    local url=$1
    local filename=$2
    local filepath="$SAMPLES_DIR/$filename"
    
    if [ -f "$filepath" ]; then
        echo -e "  ${GREEN}✓${NC} $filename already exists"
        return 0
    fi
    
    echo -ne "  ${BLUE}→${NC} Downloading $filename... "
    if timeout 30 curl -L -f -s -o "$filepath" "$url" 2>/dev/null; then
        size=$(du -h "$filepath" 2>/dev/null | cut -f1)
        echo -e "${GREEN}✓${NC} ($size)"
        return 0
    else
        rm -f "$filepath"
        return 1
    fi
}

# Direct URLs to known-working public DICOM files

echo -e "${YELLOW}Category: Reference Test Files${NC}"
# Small reference images
download_file "https://dcmjs.org/public/test/LIDC-IDRI-0010_CT.dcm" "lidc-ct.dcm" || true
download_file "https://dcmjs.org/public/test/brain_001.dcm" "brain-mri.dcm" || true
download_file "https://dcmjs.org/public/test/multiframeUS.dcm" "ultrasound-multiframe.dcm" || true

echo -e "\n${YELLOW}Category: Transfer Syntax Examples${NC}"
# JPEG Lossless
download_file "https://dcmjs.org/public/test/image_dstore.dcm" "jpeg-lossless-test.dcm" || true

echo -e "\n${YELLOW}Category: Modality Samples${NC}"
# Structured Reports
download_file "https://dcmjs.org/public/test/sr_document.dcm" "structured-report.dcm" || true

echo -e "\n${YELLOW}Category: Additional Sources${NC}"
# NIH/public medical imaging
download_file "https://www.nlm.nih.gov/research/visible/dicom/sample.dcm" "nih-sample.dcm" || true

# Summary
echo ""
echo -e "${GREEN}======== Sample File Summary ========${NC}"
echo ""
echo "Total sample files:"
find "$SAMPLES_DIR" -maxdepth 1 -type f 2>/dev/null | wc -l

echo ""
echo "Sample files list:"
find "$SAMPLES_DIR" -maxdepth 1 -type f 2>/dev/null | sort | while read f; do
    size=$(ls -lh "$f" 2>/dev/null | awk '{print $5}')
    name=$(basename "$f")
    printf "  %-35s %6s\n" "$name" "$size"
done

echo ""
echo "Total size:"
du -sh "$SAMPLES_DIR" 2>/dev/null

echo ""
echo -e "${BLUE}ℹ  For more sample sources, see DICOM-SAMPLES.md${NC}"
echo -e "${BLUE}ℹ  You can manually copy DICOM files to samples/ directory${NC}"
