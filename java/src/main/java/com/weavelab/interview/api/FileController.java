package com.weavelab.interview.api;

import com.weavelab.interview.auth.AuthFilter;
import com.weavelab.interview.model.FileMetadata;
import com.weavelab.interview.repository.ActivityRepository;
import com.weavelab.interview.repository.FileRepository;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import java.io.IOException;
import java.io.InputStream;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

@RestController
@RequestMapping("/api/files")
public class FileController {

    private final FileRepository fileRepository;
    private final ActivityRepository activityRepository;

    public FileController(FileRepository fileRepository, ActivityRepository activityRepository) {
        this.fileRepository = fileRepository;
        this.activityRepository = activityRepository;
    }

    @GetMapping
    public ResponseEntity<List<FileMetadata>> list() {
        return ResponseEntity.ok(fileRepository.list());
    }

    @PostMapping
    public ResponseEntity<?> upload(@RequestParam("file") MultipartFile file,
                                    HttpServletRequest request) {
        if (file.isEmpty()) {
            return ResponseEntity.badRequest().body(Map.of("error", "missing file field"));
        }

        String id = UUID.randomUUID().toString();
        String contentType = file.getContentType();
        if (contentType == null || contentType.isEmpty()) {
            contentType = "application/octet-stream";
        }

        try {
            FileMetadata metadata = fileRepository.create(
                    id,
                    file.getOriginalFilename(),
                    contentType,
                    file.getInputStream()
            );

            String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
            activityRepository.log(userId, "upload", "file", id);

            return ResponseEntity.status(HttpStatus.CREATED).body(metadata);
        } catch (IOException e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                    .body(Map.of("error", "failed to save file"));
        }
    }

    @GetMapping("/{id}")
    public void download(@PathVariable String id,
                         HttpServletRequest request,
                         HttpServletResponse response) throws IOException {
        Optional<FileMetadata> metadataOpt = fileRepository.findById(id);
        if (metadataOpt.isEmpty()) {
            response.setStatus(HttpServletResponse.SC_NOT_FOUND);
            response.setContentType("application/json");
            response.getWriter().write("{\"error\":\"file not found\"}");
            return;
        }

        FileMetadata metadata = metadataOpt.get();
        response.setContentType(metadata.getContentType());
        response.setHeader("Content-Disposition", "attachment; filename=" + metadata.getFilename());

        try (InputStream in = fileRepository.openFile(id)) {
            in.transferTo(response.getOutputStream());
        }

        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        activityRepository.log(userId, "download", "file", id);
    }
}
