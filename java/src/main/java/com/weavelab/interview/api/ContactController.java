package com.weavelab.interview.api;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.opencsv.CSVWriter;
import com.weavelab.interview.auth.AuthFilter;
import com.weavelab.interview.model.Contact;
import com.weavelab.interview.model.PageToken;
import com.weavelab.interview.repository.ActivityRepository;
import com.weavelab.interview.repository.ContactRepository;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.io.IOException;
import java.time.format.DateTimeFormatter;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.UUID;

@RestController
@RequestMapping("/api/contacts")
public class ContactController {

    private final ContactRepository contactRepository;
    private final ActivityRepository activityRepository;
    private final ObjectMapper objectMapper;

    private static final DateTimeFormatter ISO_FORMATTER = DateTimeFormatter.ISO_INSTANT;

    public ContactController(ContactRepository contactRepository,
                             ActivityRepository activityRepository,
                             ObjectMapper objectMapper) {
        this.contactRepository = contactRepository;
        this.activityRepository = activityRepository;
        this.objectMapper = objectMapper;
    }

    record ListContactsResponse(
            List<Contact> contacts,
            @JsonProperty("next_page_token") String nextPageToken
    ) {}

    @GetMapping
    public ResponseEntity<?> list(
            @RequestParam(defaultValue = "50") int limit,
            @RequestParam(name = "page_token", required = false) String pageTokenStr) {

        if (limit < 1 || limit > 100) {
            limit = 50;
        }

        PageToken cursor = null;
        if (pageTokenStr != null && !pageTokenStr.isEmpty()) {
            cursor = decodePageToken(pageTokenStr);
            if (cursor == null) {
                return ResponseEntity.badRequest().body(Map.of("error", "invalid page token"));
            }
        }

        List<Contact> contacts = contactRepository.list(limit + 1, cursor);

        String nextToken = null;
        if (contacts.size() > limit) {
            Contact last = contacts.get(limit - 1);
            nextToken = encodePageToken(new PageToken(last.getCreatedAt(), last.getId()));
            contacts = contacts.subList(0, limit);
        }

        return ResponseEntity.ok(new ListContactsResponse(contacts, nextToken));
    }

    @GetMapping("/{id}")
    public ResponseEntity<?> get(@PathVariable String id) {
        return contactRepository.findById(id)
                .<ResponseEntity<?>>map(ResponseEntity::ok)
                .orElseGet(() -> ResponseEntity.status(HttpStatus.NOT_FOUND)
                        .body(Map.of("error", "contact not found")));
    }

    @PostMapping
    public ResponseEntity<?> create(@RequestBody Contact contact, HttpServletRequest request) {
        contact.setId(UUID.randomUUID().toString());
        contactRepository.create(contact);

        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        activityRepository.log(userId, "create", "contact", contact.getId());

        return ResponseEntity.status(HttpStatus.CREATED).body(contact);
    }

    @PutMapping("/{id}")
    public ResponseEntity<?> update(@PathVariable String id,
                                    @RequestBody Contact contact,
                                    HttpServletRequest request) {
        contact.setId(id);
        if (!contactRepository.update(contact)) {
            return ResponseEntity.status(HttpStatus.NOT_FOUND)
                    .body(Map.of("error", "contact not found"));
        }

        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        activityRepository.log(userId, "update", "contact", id);

        return ResponseEntity.ok(contact);
    }

    @DeleteMapping("/{id}")
    public ResponseEntity<?> delete(@PathVariable String id, HttpServletRequest request) {
        if (!contactRepository.delete(id)) {
            return ResponseEntity.status(HttpStatus.NOT_FOUND)
                    .body(Map.of("error", "contact not found"));
        }

        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        activityRepository.log(userId, "delete", "contact", id);

        return ResponseEntity.noContent().build();
    }

    @PostMapping("/import")
    public ResponseEntity<?> importContacts(@RequestBody List<Contact> contacts,
                                            HttpServletRequest request) {
        if (contacts.size() > 10000) {
            return ResponseEntity.badRequest()
                    .body(Map.of("error", "maximum 10000 contacts per import"));
        }

        for (Contact c : contacts) {
            c.setId(UUID.randomUUID().toString());
        }

        int imported = contactRepository.importContacts(contacts);

        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        activityRepository.log(userId, "import", "contacts", "");

        return ResponseEntity.ok(Map.of("imported", imported));
    }

    @GetMapping("/export")
    public void export(HttpServletRequest request, HttpServletResponse response) throws IOException {
        List<Contact> contacts = contactRepository.exportAll();

        response.setContentType("text/csv");
        response.setHeader("Content-Disposition", "attachment; filename=contacts.csv");

        try (CSVWriter writer = new CSVWriter(response.getWriter())) {
            writer.writeNext(new String[]{
                    "id", "first_name", "last_name", "email", "phone", "company", "created_at", "updated_at"
            });

            for (Contact c : contacts) {
                writer.writeNext(new String[]{
                        c.getId(),
                        c.getFirstName(),
                        c.getLastName(),
                        c.getEmail(),
                        c.getPhone(),
                        c.getCompany(),
                        ISO_FORMATTER.format(c.getCreatedAt()),
                        ISO_FORMATTER.format(c.getUpdatedAt())
                });
            }
        }

        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        activityRepository.log(userId, "export", "contacts", "");
    }

    private String encodePageToken(PageToken token) {
        try {
            String json = objectMapper.writeValueAsString(token);
            return Base64.getUrlEncoder().withoutPadding().encodeToString(json.getBytes());
        } catch (JsonProcessingException e) {
            return null;
        }
    }

    private PageToken decodePageToken(String encoded) {
        try {
            byte[] decoded = Base64.getUrlDecoder().decode(encoded);
            PageToken token = objectMapper.readValue(decoded, PageToken.class);
            if (token.getCreatedAt() == null || token.getId() == null || token.getId().isEmpty()) {
                return null;
            }
            return token;
        } catch (Exception e) {
            return null;
        }
    }
}
