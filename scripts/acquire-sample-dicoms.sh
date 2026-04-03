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
DOWNLOADED_COUNT=0
SKIPPED_COUNT=0
FAILED_COUNT=0

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
        SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
        return 0
    fi
    
    echo -ne "  ${BLUE}→${NC} Downloading $filename... "
    if timeout 60 curl -L -f -s -o "$filepath" "$url" 2>/dev/null; then
        size=$(du -h "$filepath" 2>/dev/null | cut -f1)
        echo -e "${GREEN}✓${NC} ($size)"
        DOWNLOADED_COUNT=$((DOWNLOADED_COUNT + 1))
        return 0
    else
        rm -f "$filepath"
        echo -e "${YELLOW}✗${NC} unavailable"
        FAILED_COUNT=$((FAILED_COUNT + 1))
        return 1
    fi
}

download_group() {
    local title=$1
    shift

    echo -e "${YELLOW}${title}${NC}"
    for spec in "$@"; do
        local url="${spec%%|*}"
        local filename="${spec#*|}"
        download_file "$url" "$filename" || true
    done
}

download_group "Category: pydicom Reference Files" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/ExplVR_BigEnd.dcm|pydicom-ExplVR_BigEnd.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/JPEG-lossy.dcm|pydicom-JPEG-lossy.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/JPEG2000.dcm|pydicom-JPEG2000.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/JPEGLSNearLossless_08.dcm|pydicom-JPEGLSNearLossless_08.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/JPEGLSNearLossless_16.dcm|pydicom-JPEGLSNearLossless_16.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/JPGExtended.dcm|pydicom-JPGExtended.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/SC_jpeg_no_color_transform.dcm|pydicom-SC_jpeg_no_color_transform.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/SC_jpeg_no_color_transform_2.dcm|pydicom-SC_jpeg_no_color_transform_2.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/SC_rgb_jpeg.dcm|pydicom-SC_rgb_jpeg.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/SC_rgb_jpeg_app14_dcmd.dcm|pydicom-SC_rgb_jpeg_app14_dcmd.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/SC_rgb_jpeg_dcmd.dcm|pydicom-SC_rgb_jpeg_dcmd.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/SC_rgb_jpeg_dcmtk.dcm|pydicom-SC_rgb_jpeg_dcmtk.dcm" \
    "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/data/test_files/SC_rgb_jpeg_gdcm.dcm|pydicom-SC_rgb_jpeg_gdcm.dcm"

echo ""
download_group "Category: highdicom Advanced Objects" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/ct_image.dcm|highdicom-ct_image.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/dx_image.dcm|highdicom-dx_image.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/seg_image_ct_binary.dcm|highdicom-seg_image_ct_binary.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/seg_image_ct_binary_fractional.dcm|highdicom-seg_image_ct_binary_fractional.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/seg_image_ct_binary_overlap.dcm|highdicom-seg_image_ct_binary_overlap.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/seg_image_ct_binary_single_frame.dcm|highdicom-seg_image_ct_binary_single_frame.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/seg_image_ct_true_fractional.dcm|highdicom-seg_image_ct_true_fractional.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sm_annotations.dcm|highdicom-sm_annotations.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sm_image.dcm|highdicom-sm_image.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sm_image_control.dcm|highdicom-sm_image_control.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sm_image_grayscale.dcm|highdicom-sm_image_grayscale.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sm_image_grayscale_reversed.dcm|highdicom-sm_image_grayscale_reversed.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sm_image_jpegls.dcm|highdicom-sm_image_jpegls.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sm_image_jpegls_nobot.dcm|highdicom-sm_image_jpegls_nobot.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sr_document.dcm|highdicom-sr_document.dcm" \
    "https://raw.githubusercontent.com/ImagingDataCommons/highdicom/master/data/test_files/sr_document_with_multiple_groups.dcm|highdicom-sr_document_with_multiple_groups.dcm"

echo ""
download_group "Category: cornerstone3D Transfer Syntax Coverage" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CT0012.fragmented_no_bot_jpeg_baseline.51.dcm|cornerstone-CT0012.fragmented_no_bot_jpeg_baseline.51.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm|cornerstone-CTImage.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_BigEndianExplicitTransferSyntax_1.2.840.10008.1.2.2.dcm|cornerstone-CTImage-big-endian-explicit.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_DeflatedExplicitVRLittleEndianTransferSyntax_1.2.840.10008.1.2.1.99.dcm|cornerstone-CTImage-deflated-explicit-le.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEG2000LosslessOnlyTransferSyntax_1.2.840.10008.1.2.4.90.dcm|cornerstone-CTImage-jpeg2000-lossless.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEG2000TransferSyntax_1.2.840.10008.1.2.4.91.dcm|cornerstone-CTImage-jpeg2000.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEGLSLosslessTransferSyntax_1.2.840.10008.1.2.4.80.dcm|cornerstone-CTImage-jpegls-lossless.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEGLSLossyTransferSyntax_1.2.840.10008.1.2.4.81.dcm|cornerstone-CTImage-jpegls-lossy.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEGProcess14SV1TransferSyntax_1.2.840.10008.1.2.4.70.dcm|cornerstone-CTImage-jpeg-process14sv1.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEGProcess14TransferSyntax_1.2.840.10008.1.2.4.57.dcm|cornerstone-CTImage-jpeg-process14.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEGProcess1TransferSyntax_1.2.840.10008.1.2.4.50.dcm|cornerstone-CTImage-jpeg-process1.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_JPEGProcess2_4TransferSyntax_1.2.840.10008.1.2.4.51.dcm|cornerstone-CTImage-jpeg-process2-4.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_LittleEndianExplicitTransferSyntax_1.2.840.10008.1.2.1.dcm|cornerstone-CTImage-explicit-le.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_LittleEndianImplicitTransferSyntax_1.2.840.10008.1.2.dcm|cornerstone-CTImage-implicit-le.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/CTImage.dcm_RLELosslessTransferSyntax_1.2.840.10008.1.2.5.dcm|cornerstone-CTImage-rle-lossless.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/TestPattern_JPEG-Baseline_YBR422.dcm|cornerstone-TestPattern-JPEG-Baseline-YBR422.dcm" \
    "https://raw.githubusercontent.com/cornerstonejs/cornerstone3D/main/packages/dicomImageLoader/testImages/TestPattern_JPEG-Baseline_YBRFull.dcm|cornerstone-TestPattern-JPEG-Baseline-YBRFull.dcm"

# Summary
echo ""
echo -e "${GREEN}======== Sample File Summary ========${NC}"
echo ""
printf "Downloaded: %d\nSkipped:    %d\nFailed:     %d\n" "$DOWNLOADED_COUNT" "$SKIPPED_COUNT" "$FAILED_COUNT"
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
