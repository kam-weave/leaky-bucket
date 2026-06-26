package com.weavelab.interview.api;

import com.weavelab.interview.auth.AuthFilter;
import com.weavelab.interview.model.ActivityReport;
import com.weavelab.interview.repository.ActivityRepository;
import jakarta.servlet.http.HttpServletRequest;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;
import java.util.List;

@RestController
@RequestMapping("/api/reports")
public class ReportController {

    private final ActivityRepository activityRepository;

    public ReportController(ActivityRepository activityRepository) {
        this.activityRepository = activityRepository;
    }

    @GetMapping("/activity")
    public ResponseEntity<List<ActivityReport>> activityReport(
            @RequestParam(required = false) String since,
            HttpServletRequest request) {

        Instant sinceInstant = Instant.now().minus(java.time.Duration.ofDays(30));

        if (since != null && !since.isEmpty()) {
            try {
                LocalDate date = LocalDate.parse(since, DateTimeFormatter.ISO_LOCAL_DATE);
                sinceInstant = date.atStartOfDay().toInstant(ZoneOffset.UTC);
            } catch (DateTimeParseException ignored) {
            }
        }

        List<ActivityReport> report = activityRepository.generateReport(sinceInstant);

        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        activityRepository.log(userId, "view", "report", "activity");

        return ResponseEntity.ok(report);
    }
}
