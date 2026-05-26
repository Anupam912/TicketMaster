package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"
)

const baseURL = "http://localhost:8080"

var venues = []struct {
	Name     string
	Address  string
	City     string
	Capacity int
}{
	{"Madison Square Garden", "4 Pennsylvania Plaza", "New York", 20000},
	{"Staples Center", "1111 S Figueroa St", "Los Angeles", 19000},
	{"United Center", "1901 W Madison St", "Chicago", 23500},
	{"TD Garden", "100 Legends Way", "Boston", 19580},
	{"American Airlines Arena", "601 Biscayne Blvd", "Miami", 19600},
	{"Chase Center", "1 Warriors Way", "San Francisco", 18064},
	{"Barclays Center", "620 Atlantic Ave", "Brooklyn", 17732},
	{"Oracle Arena", "7000 Coliseum Way", "Oakland", 19596},
	{"Wells Fargo Center", "3601 S Broad St", "Philadelphia", 20478},
	{"Crypto.com Arena", "1111 S Figueroa St", "Los Angeles", 19068},
	{"Climate Pledge Arena", "334 1st Ave N", "Seattle", 17100},
	{"Ball Arena", "1000 Chopper Cir", "Denver", 19099},
	{"Scotiabank Arena", "40 Bay St", "Toronto", 19800},
	{"Little Caesars Arena", "2645 Woodward Ave", "Detroit", 20000},
	{"State Farm Arena", "1 State Farm Dr", "Atlanta", 18118},
}

var artists = []string{
	"Taylor Swift", "Ed Sheeran", "Coldplay", "The Weeknd", "Drake",
	"Beyoncé", "Bruno Mars", "Adele", "Post Malone", "Dua Lipa",
	"Bad Bunny", "Harry Styles", "Billie Eilish", "Travis Scott", "Kendrick Lamar",
	"Ariana Grande", "Justin Bieber", "Lady Gaga", "Rihanna", "Kanye West",
	"Imagine Dragons", "Maroon 5", "OneRepublic", "Foo Fighters", "Green Day",
	"Metallica", "AC/DC", "U2", "Guns N' Roses", "Bon Jovi",
	"Red Hot Chili Peppers", "Pearl Jam", "Linkin Park", "Nirvana Tribute", "Queen Tribute",
	"BTS", "BLACKPINK", "Stray Kids", "TWICE", "NCT 127",
	"Morgan Wallen", "Luke Combs", "Chris Stapleton", "Zach Bryan", "Tyler Childers",
	"Olivia Rodrigo", "Doja Cat", "SZA", "Tyler, The Creator", "Frank Ocean",
}

var tourNames = []string{
	"World Tour 2026", "The Eras Tour", "Mathematics Tour", "Music of the Spheres",
	"After Hours Til Dawn", "Renaissance World Tour", "24K Magic Tour",
	"30 Tour", "Runaway Tour", "Future Nostalgia Tour", "El Último Tour Del Mundo",
	"Love On Tour", "Happier Than Ever Tour", "Utopia Tour", "Big Steppers Tour",
	"Sweetener Tour", "Justice World Tour", "Chromatica Ball", "Anti World Tour",
	"Donda Experience", "Mercury World Tour", "Red Pill Blues Tour",
	"Native Tour", "Concrete and Gold", "Revolution Radio Tour",
	"WorldWired Tour", "Rock or Bust Tour", "Joshua Tree Tour",
	"Not in This Lifetime Tour", "This House Is Not for Sale Tour",
	"Unlimited Love Tour", "Gigaton Tour", "Hybrid Theory Anniversary",
	"Permission to Dance", "Born Pink", "Maniac Tour", "Ready to Be", "Neo City",
	"One Night at a Time", "World Tour", "All-American Road Show", "Burn Burn Burn",
	"GUTS World Tour", "The Scarlet Tour", "SOS Tour", "Call Me If You Get Lost",
	"Blonded Tour",
}

var eventTypes = []string{
	"Concert", "Live Performance", "Stadium Tour", "Arena Show", "Festival",
	"Acoustic Night", "Unplugged Session", "Greatest Hits Tour", "Farewell Tour",
	"Anniversary Tour", "Album Release Party", "Intimate Show", "Mega Concert",
}

type loginResponse struct {
	Token string `json:"token"`
}

type venueResponse struct {
	ID string `json:"id"`
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Login as admin
	token, err := login("admin@ticketmaster.com", "Admin@123")
	if err != nil {
		log.Fatalf("Failed to login: %v", err)
	}
	log.Println("Logged in as admin")

	// Create venues
	venueIDs := []string{}
	for _, v := range venues {
		id, err := createVenue(token, v.Name, v.Address, v.City, v.Capacity)
		if err != nil {
			log.Printf("Failed to create venue %s: %v", v.Name, err)
			continue
		}
		venueIDs = append(venueIDs, id)
		log.Printf("Created venue: %s", v.Name)
	}

	if len(venueIDs) == 0 {
		log.Fatal("No venues created")
	}

	// Create 100 events
	eventCount := 0
	startDate := time.Now().AddDate(0, 1, 0) // Start 1 month from now

	for i := 0; i < 100; i++ {
		artist := artists[rand.Intn(len(artists))]
		tour := tourNames[rand.Intn(len(tourNames))]
		eventType := eventTypes[rand.Intn(len(eventTypes))]
		venueID := venueIDs[rand.Intn(len(venueIDs))]

		title := fmt.Sprintf("%s - %s", artist, tour)
		description := fmt.Sprintf("%s presents %s. A spectacular %s you won't want to miss! "+
			"Experience the magic of live music with incredible stage production, "+
			"stunning visuals, and unforgettable performances.", artist, tour, eventType)

		// Random date within next 12 months
		eventDate := startDate.AddDate(0, 0, rand.Intn(365))
		// Random time between 6 PM and 9 PM
		hour := 18 + rand.Intn(4)
		eventDate = time.Date(eventDate.Year(), eventDate.Month(), eventDate.Day(),
			hour, 0, 0, 0, time.UTC)

		// Random price between $50 and $500
		price := float64(50 + rand.Intn(450))

		// Random seats between 100 and 2000
		seats := 100 + rand.Intn(1900)

		err := createEvent(token, title, description, venueID, eventDate, price, seats)
		if err != nil {
			log.Printf("Failed to create event %s: %v", title, err)
			continue
		}
		eventCount++
		log.Printf("Created event %d: %s (%d seats, $%.2f)", eventCount, title, seats, price)
	}

	log.Printf("\n=== SEED COMPLETE ===")
	log.Printf("Created %d venues", len(venueIDs))
	log.Printf("Created %d events", eventCount)
}

func login(email, password string) (string, error) {
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(baseURL+"/api/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed: %s", string(respBody))
	}

	var result loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Token, nil
}

func createVenue(token, name, address, city string, capacity int) (string, error) {
	payload := map[string]interface{}{
		"name":     name,
		"address":  address,
		"city":     city,
		"capacity": capacity,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/api/venues", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create venue failed: %s", string(respBody))
	}

	var result venueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

func createEvent(token, title, description, venueID string, eventDate time.Time, price float64, seats int) error {
	payload := map[string]interface{}{
		"title":        title,
		"description":  description,
		"venue_id":     venueID,
		"event_date":   eventDate.Format(time.RFC3339),
		"ticket_price": price,
		"total_seats":  seats,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/api/events", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create event failed: %s", string(respBody))
	}

	return nil
}
